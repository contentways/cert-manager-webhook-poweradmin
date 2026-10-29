// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

//go:build conformance

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	acmetest "github.com/cert-manager/cert-manager/test/acme"
	"github.com/contentways/poweradmin-go/v4/poweradmin"
	"github.com/miekg/dns"

	"github.com/contentways/cert-manager-webhook-poweradmin/internal/solver"
)

// TestConformance runs the cert-manager DNS01 webhook conformance suite
// against a real Poweradmin. It needs:
//
//	POWERADMIN_URL         Poweradmin base URL, e.g. http://127.0.0.1:8080
//	POWERADMIN_API_KEY     API key with access to the zone
//	POWERADMIN_ZONE        zone the test records go to, e.g. example.com
//	POWERADMIN_DNS_SERVER  optional PowerDNS host:port serving the zone; without
//	                       it a DNS shim answers from the Poweradmin API
//
// and the envtest binaries in TEST_ASSET_ETCD, TEST_ASSET_KUBE_APISERVER and
// TEST_ASSET_KUBECTL (see make test-conformance).
func TestConformance(t *testing.T) {
	baseURL, apiKey := os.Getenv("POWERADMIN_URL"), os.Getenv("POWERADMIN_API_KEY")
	zone := strings.TrimSuffix(os.Getenv("POWERADMIN_ZONE"), ".")
	if baseURL == "" || apiKey == "" || zone == "" {
		t.Skip("POWERADMIN_URL, POWERADMIN_API_KEY and POWERADMIN_ZONE must be set")
	}

	dnsServer := os.Getenv("POWERADMIN_DNS_SERVER")
	if dnsServer == "" {
		dnsServer = startAPIBackedDNS(t, baseURL, apiKey, zone)
	}

	manifests := t.TempDir()
	secret := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: poweradmin
type: Opaque
stringData:
  api-key: %q
`, apiKey)
	if err := os.WriteFile(filepath.Join(manifests, "secret.yaml"), []byte(secret), 0o600); err != nil {
		t.Fatal(err)
	}

	fixture := acmetest.NewFixture(solver.New(),
		acmetest.SetResolvedZone(zone+"."),
		acmetest.SetManifestPath(manifests),
		acmetest.SetConfig(map[string]any{
			"baseURL":         baseURL,
			"apiKeySecretRef": map[string]any{"name": "poweradmin", "key": "api-key"},
			"ttl":             60,
		}),
		acmetest.SetDNSServer(dnsServer),
		acmetest.SetUseAuthoritative(false),
		acmetest.SetStrict(true),
		acmetest.SetPollInterval(time.Second),
		acmetest.SetPropagationLimit(time.Minute),
	)
	fixture.RunConformance(t)
}

// startAPIBackedDNS serves the TXT records of zone over UDP by reading them
// from the Poweradmin API, standing in for PowerDNS.
func startAPIBackedDNS(t *testing.T, baseURL, apiKey, zone string) string {
	t.Helper()
	client, err := poweradmin.NewClient(poweradmin.WithBaseURL(baseURL), poweradmin.WithAPIKey(apiKey))
	if err != nil {
		t.Fatal(err)
	}
	z, _, err := client.Zone.GetByName(t.Context(), zone)
	if err != nil {
		t.Fatalf("looking up zone %s: %v", zone, err)
	}

	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &dns.Server{PacketConn: conn, Handler: dns.HandlerFunc(func(w dns.ResponseWriter, req *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(req)
		m.Authoritative = true
		for _, q := range req.Question {
			name := strings.ToLower(strings.TrimSuffix(q.Name, "."))
			if q.Qtype != dns.TypeTXT || (name != zone && !strings.HasSuffix(name, "."+zone)) {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			rrset, _, err := client.RRSet.Get(ctx, z.ID, name, "TXT")
			cancel()
			if poweradmin.IsNotFound(err) {
				m.Rcode = dns.RcodeNameError
				continue
			}
			if err != nil {
				m.Rcode = dns.RcodeServerFailure
				continue
			}
			for _, r := range rrset.Records {
				m.Answer = append(m.Answer, &dns.TXT{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: uint32(rrset.TTL)},
					Txt: []string{strings.Trim(r.Content, `"`)},
				})
			}
		}
		_ = w.WriteMsg(m)
	})}
	go func() { _ = srv.ActivateAndServe() }()
	t.Cleanup(func() { _ = srv.Shutdown() })
	return conn.LocalAddr().String()
}

// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

package solver

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

const testNamespace = "customer-a"

func secret(namespace, name, key, value string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Data:       map[string][]byte{key: []byte(value)},
	}
}

func rawConfig(t *testing.T, cfg map[string]any) *extapi.JSON {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &extapi.JSON{Raw: raw}
}

func baseConfig(url string) map[string]any {
	return map[string]any{
		"baseURL":         url,
		"apiKeySecretRef": map[string]any{"name": "poweradmin", "key": "api-key"},
	}
}

func challenge(t *testing.T, cfg map[string]any, fqdn, zone, key string) *v1alpha1.ChallengeRequest {
	return &v1alpha1.ChallengeRequest{
		Action:            v1alpha1.ChallengeActionPresent,
		ResolvedFQDN:      fqdn,
		ResolvedZone:      zone,
		Key:               key,
		ResourceNamespace: testNamespace,
		Config:            rawConfig(t, cfg),
	}
}

func newTestSolver(t *testing.T, zones ...string) (*Solver, *fakeAPI, string) {
	t.Helper()
	api, srv := newFakeAPI(t, zones...)
	kube := fake.NewClientset(secret(testNamespace, "poweradmin", "api-key", "test-key\n"))
	return NewWithClient(kube, srv.Client()), api, srv.URL
}

func TestPresentCreatesRecordWithDefaultTTL(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")

	if err := s.Present(challenge(t, baseConfig(url), "_acme-challenge.www.example.com.", "example.com.", "key-1")); err != nil {
		t.Fatalf("Present: %v", err)
	}

	rr, ok := api.rrset(1, "_acme-challenge.www.example.com")
	if !ok {
		t.Fatal("TXT RRset was not created")
	}
	if rr.TTL != DefaultTTL || len(rr.Records) != 1 || rr.Records[0].Content != "key-1" {
		t.Errorf("rrset = %+v", rr)
	}
}

func TestPresentIsIdempotent(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	ch := challenge(t, baseConfig(url), "_acme-challenge.example.com.", "example.com.", "key-1")

	for range 3 {
		if err := s.Present(ch); err != nil {
			t.Fatalf("Present: %v", err)
		}
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"key-1"}) {
		t.Errorf("values = %v, want one key", got)
	}
	if api.puts != 1 {
		t.Errorf("PUT requests = %d, want 1", api.puts)
	}
}

// Apex and wildcard certificates share _acme-challenge.<domain>.
func TestPresentKeepsOtherValuesAndCleanUpRemovesOnlyItsOwn(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	cfg := baseConfig(url)
	fqdn := "_acme-challenge.example.com."

	for _, key := range []string{"apex-key", "wildcard-key"} {
		if err := s.Present(challenge(t, cfg, fqdn, "example.com.", key)); err != nil {
			t.Fatalf("Present %s: %v", key, err)
		}
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"apex-key", "wildcard-key"}) {
		t.Fatalf("values after Present = %v", got)
	}

	if err := s.CleanUp(challenge(t, cfg, fqdn, "example.com.", "apex-key")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"wildcard-key"}) {
		t.Fatalf("values after first CleanUp = %v", got)
	}

	if err := s.CleanUp(challenge(t, cfg, fqdn, "example.com.", "wildcard-key")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if _, ok := api.rrset(1, "_acme-challenge.example.com"); ok {
		t.Error("RRset still exists after the last value was cleaned up")
	}
}

func TestCleanUpOfMissingRecordSucceeds(t *testing.T) {
	s, _, url := newTestSolver(t, "example.com")
	if err := s.CleanUp(challenge(t, baseConfig(url), "_acme-challenge.example.com.", "example.com.", "key-1")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
}

func TestCleanUpKeepsRecordsItDidNotCreate(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	api.seed(1, fakeRRSet{Name: "_acme-challenge.example.com", Type: "TXT", TTL: 300,
		Records: []fakeRecordEntry{{Content: "foreign"}}})

	if err := s.CleanUp(challenge(t, baseConfig(url), "_acme-challenge.example.com.", "example.com.", "key-1")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"foreign"}) {
		t.Errorf("values = %v, want the foreign value untouched", got)
	}
	if api.puts != 0 {
		t.Errorf("PUT requests = %d, want none", api.puts)
	}
}

func TestQuotedContentsFromTheServerMatch(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	api.quoteContents = true
	cfg := baseConfig(url)
	fqdn := "_acme-challenge.example.com."

	for _, key := range []string{"key-1", "key-2"} {
		if err := s.Present(challenge(t, cfg, fqdn, "example.com.", key)); err != nil {
			t.Fatalf("Present: %v", err)
		}
	}
	// Presenting again must recognise the quoted value as present
	if err := s.Present(challenge(t, cfg, fqdn, "example.com.", "key-1")); err != nil {
		t.Fatalf("Present: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"key-1", "key-2"}) {
		t.Fatalf("values = %v", got)
	}
	if err := s.CleanUp(challenge(t, cfg, fqdn, "example.com.", "key-1")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"key-2"}) {
		t.Errorf("values = %v", got)
	}
}

func TestPresentKeepsTTLOfExistingRRSetOnCleanUp(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	cfg := baseConfig(url)
	cfg["ttl"] = 60
	fqdn := "_acme-challenge.example.com."

	for _, key := range []string{"key-1", "key-2"} {
		if err := s.Present(challenge(t, cfg, fqdn, "example.com.", key)); err != nil {
			t.Fatalf("Present: %v", err)
		}
	}
	if err := s.CleanUp(challenge(t, cfg, fqdn, "example.com.", "key-1")); err != nil {
		t.Fatalf("CleanUp: %v", err)
	}
	if rr, _ := api.rrset(1, "_acme-challenge.example.com"); rr.TTL != 60 {
		t.Errorf("TTL = %d, want 60", rr.TTL)
	}
}

func TestZoneResolution(t *testing.T) {
	tests := []struct {
		name     string
		zones    []string
		fqdn     string
		resolved string
		override string
		wantZone int
		wantErr  string
	}{
		{name: "resolved zone", zones: []string{"example.com"}, fqdn: "_acme-challenge.www.example.com.", resolved: "example.com.", wantZone: 1},
		{name: "most specific parent when the resolved zone is unknown", zones: []string{"example.com", "dev.example.com"},
			fqdn: "_acme-challenge.app.dev.example.com.", resolved: "app.dev.example.com.", wantZone: 2},
		{name: "configured zone wins", zones: []string{"example.com", "acme.example.net"},
			fqdn: "_acme-challenge.acme.example.net.", resolved: "example.net.", override: "acme.example.net.", wantZone: 2},
		{name: "upper case names", zones: []string{"example.com"}, fqdn: "_ACME-challenge.WWW.Example.COM.", resolved: "Example.COM.", wantZone: 1},
		{name: "no zone", zones: []string{"example.org"}, fqdn: "_acme-challenge.example.com.", resolved: "example.com.",
			wantErr: "no Poweradmin zone found for _acme-challenge.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, api, url := newTestSolver(t, tt.zones...)
			cfg := baseConfig(url)
			if tt.override != "" {
				cfg["zone"] = tt.override
			}
			err := s.Present(challenge(t, cfg, tt.fqdn, tt.resolved, "key"))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Present: %v", err)
			}
			if got := api.values(tt.wantZone, strings.ToLower(strings.TrimSuffix(tt.fqdn, "."))); len(got) != 1 {
				t.Errorf("record not in zone %d: %v", tt.wantZone, got)
			}
		})
	}
}

func TestSecretIsReadFromTheIssuerNamespace(t *testing.T) {
	api, srv := newFakeAPI(t, "example.com")
	kube := fake.NewClientset(
		secret("customer-b", "poweradmin", "api-key", "test-key"),
		secret(testNamespace, "poweradmin", "api-key", "wrong-key"),
	)
	s := NewWithClient(kube, srv.Client())

	err := s.Present(challenge(t, baseConfig(srv.URL), "_acme-challenge.example.com.", "example.com.", "key"))
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want the 401 of customer-a's key", err)
	}

	ch := challenge(t, baseConfig(srv.URL), "_acme-challenge.example.com.", "example.com.", "key")
	ch.ResourceNamespace = "customer-b"
	if err := s.Present(ch); err != nil {
		t.Fatalf("Present with customer-b's key: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); len(got) != 1 {
		t.Errorf("values = %v", got)
	}
}

func TestSecretErrors(t *testing.T) {
	tests := []struct {
		name    string
		objects []*corev1.Secret
		wantErr string
	}{
		{name: "missing secret", wantErr: "reading Secret customer-a/poweradmin"},
		{name: "missing key", objects: []*corev1.Secret{secret(testNamespace, "poweradmin", "other", "x")}, wantErr: `has no key "api-key"`},
		{name: "empty key", objects: []*corev1.Secret{secret(testNamespace, "poweradmin", "api-key", " \n")}, wantErr: "is empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, srv := newFakeAPI(t, "example.com")
			kube := fake.NewClientset()
			for _, o := range tt.objects {
				if _, err := kube.CoreV1().Secrets(o.Namespace).Create(t.Context(), o, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			s := NewWithClient(kube, srv.Client())
			err := s.Present(challenge(t, baseConfig(srv.URL), "_acme-challenge.example.com.", "example.com.", "key"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestConcurrentPresentsForTheSameName(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	cfg := baseConfig(url)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Go(func() {
			errs <- s.Present(challenge(t, cfg, "_acme-challenge.example.com.", "example.com.", fmt.Sprintf("key-%d", i)))
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("Present: %v", err)
		}
	}
	if got := api.values(1, "_acme-challenge.example.com"); len(got) != 10 {
		t.Errorf("values = %v, want all 10 keys", got)
	}
}

// A write from another webhook replica between our read and write is detected
// by reading the record back, and the write is repeated.
func TestPresentRetriesWhenAConcurrentWriteWins(t *testing.T) {
	s, api, url := newTestSolver(t, "example.com")
	gets := 0
	api.onGet = func() {
		gets++
		if gets == 2 { // the read-back after our first PUT
			api.mu.Lock()
			api.rrsets[1]["_acme-challenge.example.com|TXT"] = fakeRRSet{Name: "_acme-challenge.example.com", Type: "TXT", TTL: 120,
				Records: []fakeRecordEntry{{Content: "other-replica"}}}
			api.mu.Unlock()
		}
	}

	if err := s.Present(challenge(t, baseConfig(url), "_acme-challenge.example.com.", "example.com.", "key-1")); err != nil {
		t.Fatalf("Present: %v", err)
	}
	if got := api.values(1, "_acme-challenge.example.com"); !slices.Equal(got, []string{"other-replica", "key-1"}) {
		t.Errorf("values = %v", got)
	}
}

func TestClientsAreCachedPerConfigAndKey(t *testing.T) {
	s := New()
	cfg := Config{BaseURL: "https://dns.example.com"}
	a, _ := s.client(cfg, "key-a")
	b, _ := s.client(cfg, "key-a")
	c, _ := s.client(cfg, "key-b")
	if a != b {
		t.Error("same URL and key built a second client")
	}
	if a == c {
		t.Error("a different key reused the client")
	}
}

func TestCABundleMustContainACertificate(t *testing.T) {
	s := New()
	if _, err := s.client(Config{BaseURL: "https://dns.example.com", CABundle: "not a pem"}, "key"); err == nil {
		t.Fatal("want an error for a CA bundle without a certificate")
	}
}

func TestNameAndUninitialized(t *testing.T) {
	s := New()
	if s.Name() != "poweradmin" {
		t.Errorf("Name = %q", s.Name())
	}
	err := s.Present(challenge(t, baseConfig("https://dns.example.com"), "_acme-challenge.example.com.", "example.com.", "k"))
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("err = %v", err)
	}
}

// The conformance suite and webhook restarts call Initialize again with a new
// API server; the solver must not keep the old client.
func TestInitializeReplacesTheKubeClient(t *testing.T) {
	s := New()
	if err := s.Initialize(&rest.Config{Host: "https://first.example"}, nil); err != nil {
		t.Fatal(err)
	}
	first := s.kube
	if err := s.Initialize(&rest.Config{Host: "https://second.example"}, nil); err != nil {
		t.Fatal(err)
	}
	if s.kube == first {
		t.Error("Initialize kept the client of the first API server")
	}
}

// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

// Package solver implements the cert-manager ACME DNS01 webhook solver for
// Poweradmin. Challenge records are written through the Poweradmin API v2
// RRset endpoints, which address records by name and type: record IDs are not
// stable when Poweradmin runs on the PowerDNS API backend.
package solver

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/contentways/poweradmin-go/v4/poweradmin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Name is the solver name referenced by issuers in
// spec.acme.solvers[].dns01.webhook.solverName.
const Name = "poweradmin"

const (
	recordType     = "TXT"
	requestTimeout = 30 * time.Second
	// Poweradmin rewrites an RRset as a whole, so two challenges for the same
	// name (apex and wildcard) are a read-modify-write race. After writing, the
	// record is read back and the write repeated if a concurrent one won.
	maxWriteAttempts = 3
)

// Solver is the Poweradmin DNS01 solver.
type Solver struct {
	kube kubernetes.Interface
	// injected keeps a kube client from NewWithClient across Initialize.
	injected bool

	// httpClient is used for every Poweradmin client the solver builds,
	// unless the issuer configures its own trust settings. Tests replace it.
	httpClient *http.Client

	mu      sync.Mutex
	clients map[string]*poweradmin.Client
	locks   map[string]*sync.Mutex
}

var _ webhook.Solver = (*Solver)(nil)

// New returns a solver; Initialize connects it to Kubernetes.
func New() *Solver {
	return &Solver{
		clients: map[string]*poweradmin.Client{},
		locks:   map[string]*sync.Mutex{},
	}
}

// NewWithClient returns a solver that reads Secrets through the given
// Kubernetes client, for tests.
func NewWithClient(kube kubernetes.Interface, httpClient *http.Client) *Solver {
	s := New()
	s.kube = kube
	s.injected = true
	s.httpClient = httpClient
	return s
}

// Name implements webhook.Solver.
func (s *Solver) Name() string {
	return Name
}

// Initialize implements webhook.Solver.
func (s *Solver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	if s.injected {
		return nil
	}
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("creating Kubernetes client: %w", err)
	}
	s.kube = cl
	return nil
}

// Present adds the challenge key to the TXT RRset of ResolvedFQDN. Other
// values in the RRset, such as the key of a concurrent challenge for the same
// name, are kept, and presenting the same key twice is a no-op.
func (s *Solver) Present(ch *v1alpha1.ChallengeRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	target, err := s.prepare(ctx, ch)
	if err != nil {
		return err
	}

	unlock := s.lock(target.key())
	defer unlock()

	for attempt := 1; ; attempt++ {
		rrset, err := target.get(ctx)
		if err != nil {
			return err
		}
		if rrset != nil && containsValue(rrset.Records, ch.Key) {
			klog.V(2).InfoS("challenge record already present", "fqdn", target.name, "zone", target.zone.Name)
			return nil
		}

		ttl := target.cfg.ttl()
		var records []poweradmin.RRSetRecord
		if rrset != nil {
			records = rrset.Records
		}
		records = append(slices.Clone(records), poweradmin.RRSetRecord{Content: ch.Key})
		if err := target.put(ctx, ttl, records); err != nil {
			return err
		}

		ok, err := target.has(ctx, ch.Key)
		if err != nil {
			return err
		}
		if ok {
			klog.InfoS("presented challenge record", "fqdn", target.name, "zone", target.zone.Name, "namespace", ch.ResourceNamespace)
			return nil
		}
		if attempt == maxWriteAttempts {
			return fmt.Errorf("challenge record for %s did not persist after %d attempts", target.name, attempt)
		}
	}
}

// CleanUp removes only this challenge's key from the TXT RRset and deletes the
// RRset once no value is left. A record that is already gone is not an error.
func (s *Solver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	target, err := s.prepare(ctx, ch)
	if err != nil {
		return err
	}

	unlock := s.lock(target.key())
	defer unlock()

	for attempt := 1; ; attempt++ {
		rrset, err := target.get(ctx)
		if err != nil {
			return err
		}
		if rrset == nil || !containsValue(rrset.Records, ch.Key) {
			return nil
		}

		remaining := slices.DeleteFunc(slices.Clone(rrset.Records), func(r poweradmin.RRSetRecord) bool {
			return sameValue(r.Content, ch.Key)
		})
		if len(remaining) == 0 {
			if _, err := target.client.RRSet.Delete(ctx, target.zone.ID, target.name, recordType); err != nil && !poweradmin.IsNotFound(err) {
				return fmt.Errorf("deleting TXT %s in zone %s: %w", target.name, target.zone.Name, err)
			}
		} else if err := target.put(ctx, rrset.TTL, remaining); err != nil {
			return err
		}

		ok, err := target.has(ctx, ch.Key)
		if err != nil {
			return err
		}
		if !ok {
			klog.InfoS("cleaned up challenge record", "fqdn", target.name, "zone", target.zone.Name, "namespace", ch.ResourceNamespace)
			return nil
		}
		if attempt == maxWriteAttempts {
			return fmt.Errorf("challenge record for %s was still present after %d attempts", target.name, attempt)
		}
	}
}

// target is the zone and record name a challenge writes to.
type target struct {
	cfg    Config
	client *poweradmin.Client
	zone   *poweradmin.Zone
	name   string
}

func (t *target) key() string {
	return t.cfg.BaseURL + "|" + t.name
}

// get returns the TXT RRset, or nil when the name has none.
func (t *target) get(ctx context.Context) (*poweradmin.RRSet, error) {
	rrset, _, err := t.client.RRSet.Get(ctx, t.zone.ID, t.name, recordType)
	if poweradmin.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading TXT %s in zone %s: %w", t.name, t.zone.Name, err)
	}
	return rrset, nil
}

func (t *target) has(ctx context.Context, value string) (bool, error) {
	rrset, err := t.get(ctx)
	if err != nil {
		return false, err
	}
	return rrset != nil && containsValue(rrset.Records, value), nil
}

func (t *target) put(ctx context.Context, ttl int, records []poweradmin.RRSetRecord) error {
	for i := range records {
		records[i].Content = unquote(records[i].Content)
	}
	_, err := t.client.RRSet.Upsert(ctx, t.zone.ID, poweradmin.RRSetUpsertOpts{
		Name:    t.name,
		Type:    recordType,
		TTL:     ttl,
		Records: records,
	})
	if err != nil {
		return fmt.Errorf("writing TXT %s in zone %s: %w", t.name, t.zone.Name, err)
	}
	return nil
}

// prepare loads the issuer configuration and API key and finds the zone.
func (s *Solver) prepare(ctx context.Context, ch *v1alpha1.ChallengeRequest) (*target, error) {
	cfg, err := LoadConfig(ch.Config)
	if err != nil {
		return nil, err
	}
	apiKey, err := s.apiKey(ctx, ch.ResourceNamespace, cfg)
	if err != nil {
		return nil, err
	}
	client, err := s.client(cfg, apiKey)
	if err != nil {
		return nil, err
	}

	name := normalize(ch.ResolvedFQDN)
	zone, err := findZone(ctx, client, cfg.Zone, normalize(ch.ResolvedZone), name)
	if err != nil {
		return nil, err
	}

	return &target{cfg: cfg, client: client, zone: zone, name: name}, nil
}

func (s *Solver) apiKey(ctx context.Context, namespace string, cfg Config) (string, error) {
	if s.kube == nil {
		return "", errors.New("solver is not initialized")
	}
	ref := cfg.APIKeySecretRef
	secret, err := s.kube.CoreV1().Secrets(namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("reading Secret %s/%s: %w", namespace, ref.Name, err)
	}
	value, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no key %q", namespace, ref.Name, ref.Key)
	}
	key := strings.TrimSpace(string(value))
	if key == "" {
		return "", fmt.Errorf("key %q in Secret %s/%s is empty", ref.Key, namespace, ref.Name)
	}
	return key, nil
}

// client returns a cached Poweradmin client for the URL, key and trust
// settings, so a renewal storm does not build a client per challenge.
func (s *Solver) client(cfg Config, apiKey string) (*poweradmin.Client, error) {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		cfg.BaseURL, apiKey, cfg.CABundle, fmt.Sprint(cfg.InsecureSkipVerify),
	}, "\x00")))
	cacheKey := hex.EncodeToString(sum[:])

	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.clients[cacheKey]; ok {
		return c, nil
	}

	opts := []poweradmin.Option{
		poweradmin.WithBaseURL(cfg.BaseURL),
		poweradmin.WithAPIKey(apiKey),
		poweradmin.WithRetry(3),
	}
	switch {
	case cfg.CABundle != "" || cfg.InsecureSkipVerify:
		hc, err := httpClientFor(cfg)
		if err != nil {
			return nil, err
		}
		opts = append(opts, poweradmin.WithHTTPClient(hc))
	case s.httpClient != nil:
		opts = append(opts, poweradmin.WithHTTPClient(s.httpClient))
	}

	c, err := poweradmin.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Poweradmin client: %w", err)
	}
	s.clients[cacheKey] = c
	return c, nil
}

func httpClientFor(cfg Config) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.CABundle != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(cfg.CABundle)) {
			return nil, errors.New("caBundle contains no PEM certificate")
		}
		tlsConfig.RootCAs = pool
	}
	if cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true //nolint:gosec // explicitly requested by the issuer
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	return &http.Client{Timeout: requestTimeout, Transport: transport}, nil
}

// lock serializes writes to one record name within this webhook instance.
func (s *Solver) lock(key string) func() {
	s.mu.Lock()
	l, ok := s.locks[key]
	if !ok {
		l = &sync.Mutex{}
		s.locks[key] = l
	}
	s.mu.Unlock()
	l.Lock()
	return l.Unlock
}

// findZone returns the Poweradmin zone for the record: the configured zone,
// otherwise the zone cert-manager resolved, otherwise the longest parent of
// the record name that Poweradmin manages.
func findZone(ctx context.Context, client *poweradmin.Client, configured, resolved, fqdn string) (*poweradmin.Zone, error) {
	var candidates []string
	if configured != "" {
		candidates = []string{configured}
	} else {
		if resolved != "" {
			candidates = append(candidates, resolved)
		}
		labels := strings.Split(fqdn, ".")
		for i := 1; i < len(labels)-1; i++ {
			c := strings.Join(labels[i:], ".")
			if c != resolved {
				candidates = append(candidates, c)
			}
		}
	}

	for _, name := range candidates {
		zone, _, err := client.Zone.GetByName(ctx, name)
		if err == nil {
			return zone, nil
		}
		if !poweradmin.IsNotFound(err) {
			return nil, fmt.Errorf("looking up zone %s: %w", name, err)
		}
	}
	return nil, fmt.Errorf("no Poweradmin zone found for %s (tried %s); check that the API key may access the zone", fqdn, strings.Join(candidates, ", "))
}

// normalize turns a DNS name into the form Poweradmin stores: lower case,
// without the trailing dot.
func normalize(name string) string {
	return strings.ToLower(strings.TrimSuffix(name, "."))
}

func containsValue(records []poweradmin.RRSetRecord, value string) bool {
	return slices.ContainsFunc(records, func(r poweradmin.RRSetRecord) bool {
		return sameValue(r.Content, value)
	})
}

// sameValue compares TXT contents with and without the surrounding quotes:
// Poweradmin returns them unquoted, PowerDNS stores them quoted.
func sameValue(a, b string) bool {
	return unquote(a) == unquote(b)
}

func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}

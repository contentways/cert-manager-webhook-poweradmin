// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

package solver

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// DefaultTTL is the TTL of a new challenge record when the issuer sets none.
const DefaultTTL = 120

// Config is the solver configuration from the issuer, under
// spec.acme.solvers[].dns01.webhook.config.
type Config struct {
	// BaseURL is the Poweradmin URL without /api, e.g. https://dns.example.com.
	BaseURL string `json:"baseURL"`

	// APIKeySecretRef names the Secret holding the Poweradmin API key. It is
	// read from the issuer's namespace, or from the cluster resource namespace
	// of cert-manager for a ClusterIssuer.
	APIKeySecretRef cmmeta.SecretKeySelector `json:"apiKeySecretRef"`

	// TTL of the challenge record in seconds; DefaultTTL when unset.
	TTL *int `json:"ttl,omitempty"`

	// Zone overrides the Poweradmin zone the record is written to, for setups
	// where the zone found by cert-manager is not the one Poweradmin manages.
	Zone string `json:"zone,omitempty"`

	// CABundle is a PEM bundle of certificate authorities trusted for BaseURL,
	// for a Poweradmin behind an internal CA.
	CABundle string `json:"caBundle,omitempty"`

	// InsecureSkipVerify disables TLS verification. For testing only.
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// LoadConfig decodes and validates the issuer's solver configuration.
func LoadConfig(raw *extapi.JSON) (Config, error) {
	var cfg Config
	if raw == nil || len(raw.Raw) == 0 {
		return cfg, errors.New("webhook config is missing: baseURL and apiKeySecretRef are required")
	}
	if err := json.Unmarshal(raw.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("decoding webhook config: %w", err)
	}

	var errs []error
	if cfg.BaseURL == "" {
		errs = append(errs, errors.New("baseURL is required"))
	} else if u, err := url.Parse(cfg.BaseURL); err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		errs = append(errs, fmt.Errorf("baseURL %q must be an absolute http or https URL", cfg.BaseURL))
	}
	if cfg.APIKeySecretRef.Name == "" {
		errs = append(errs, errors.New("apiKeySecretRef.name is required"))
	}
	if cfg.APIKeySecretRef.Key == "" {
		errs = append(errs, errors.New("apiKeySecretRef.key is required"))
	}
	if cfg.TTL != nil && *cfg.TTL < 1 {
		errs = append(errs, fmt.Errorf("ttl must be positive, got %d", *cfg.TTL))
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.Zone = strings.TrimSuffix(cfg.Zone, ".")

	return cfg, errors.Join(errs...)
}

func (c Config) ttl() int {
	if c.TTL != nil {
		return *c.TTL
	}
	return DefaultTTL
}

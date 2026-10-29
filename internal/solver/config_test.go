// Copyright (c) 2026 contentways
// SPDX-License-Identifier: MIT

package solver

import (
	"strings"
	"testing"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr []string
		check   func(t *testing.T, c Config)
	}{
		{
			name: "complete",
			raw:  `{"baseURL":"https://dns.example.com/","apiKeySecretRef":{"name":"pa","key":"api-key"},"ttl":60,"zone":"example.com."}`,
			check: func(t *testing.T, c Config) {
				if c.BaseURL != "https://dns.example.com" || c.ttl() != 60 || c.Zone != "example.com" {
					t.Errorf("config = %+v", c)
				}
			},
		},
		{
			name: "default ttl",
			raw:  `{"baseURL":"http://poweradmin.dns.svc","apiKeySecretRef":{"name":"pa","key":"api-key"}}`,
			check: func(t *testing.T, c Config) {
				if c.ttl() != DefaultTTL {
					t.Errorf("ttl = %d", c.ttl())
				}
			},
		},
		{name: "empty", raw: ``, wantErr: []string{"webhook config is missing"}},
		{name: "not json", raw: `{`, wantErr: []string{"decoding webhook config"}},
		{name: "nothing set", raw: `{}`, wantErr: []string{"baseURL is required", "apiKeySecretRef.name is required", "apiKeySecretRef.key is required"}},
		{name: "relative url", raw: `{"baseURL":"dns.example.com","apiKeySecretRef":{"name":"pa","key":"k"}}`, wantErr: []string{"must be an absolute http or https URL"}},
		{name: "bad ttl", raw: `{"baseURL":"https://dns.example.com","apiKeySecretRef":{"name":"pa","key":"k"},"ttl":0}`, wantErr: []string{"ttl must be positive"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var raw *extapi.JSON
			if tt.raw != "" {
				raw = &extapi.JSON{Raw: []byte(tt.raw)}
			}
			cfg, err := LoadConfig(raw)
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatal("want an error")
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

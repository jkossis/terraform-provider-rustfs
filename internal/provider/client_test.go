// Copyright IBM Corp. 2021, 2025
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalizeEndpoint(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		input       string
		endpoint    string
		secure      bool
		expectError bool
	}{
		"host only defaults secure": {
			input:    "rustfs.example.com:9000",
			endpoint: "rustfs.example.com:9000",
			secure:   true,
		},
		"https endpoint": {
			input:    "https://rustfs.example.com:9000",
			endpoint: "rustfs.example.com:9000",
			secure:   true,
		},
		"http endpoint": {
			input:    "http://localhost:9000",
			endpoint: "localhost:9000",
			secure:   false,
		},
		"path rejected": {
			input:       "https://rustfs.example.com:9000/admin",
			expectError: true,
		},
		"query rejected": {
			input:       "https://rustfs.example.com:9000?x=y",
			expectError: true,
		},
		"unsupported scheme rejected": {
			input:       "ftp://rustfs.example.com:9000",
			expectError: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			endpoint, secure, err := normalizeEndpoint(testCase.input)
			if testCase.expectError {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if endpoint != testCase.endpoint {
				t.Fatalf("expected endpoint %q, got %q", testCase.endpoint, endpoint)
			}

			if secure != testCase.secure {
				t.Fatalf("expected secure %t, got %t", testCase.secure, secure)
			}
		})
	}
}

func TestNewAdminRequestUsesRustFSAdminPath(t *testing.T) {
	t.Parallel()

	client := &rustfsClient{
		endpoint:  "rustfs.example.com:9000",
		secure:    true,
		accessKey: "access",
		secretKey: "secret",
	}

	req, err := client.newAdminRequest(t.Context(), "GET", rustfsAdminV3Prefix+"/site-replication/info", siteReplicationBaseQuery(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if req.URL.Path != "/rustfs/admin/v3/site-replication/info" {
		t.Fatalf("expected RustFS admin path, got %q", req.URL.Path)
	}

	if req.URL.Query().Get("api-version") != "1" {
		t.Fatalf("expected api-version query parameter")
	}

	if req.Header.Get("Authorization") == "" {
		t.Fatalf("expected signed request")
	}
}

func TestSiteReplicationAddUsesPlainJSONRustFSPath(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/rustfs/admin/v3/site-replication/add" {
			t.Errorf("expected RustFS admin path, got %q", req.URL.Path)
		}
		if req.URL.Query().Get("api-version") != "1" {
			t.Errorf("expected api-version query parameter")
		}
		if req.URL.Query().Get("replicateILMExpiry") != "true" {
			t.Errorf("expected replicateILMExpiry query parameter")
		}
		if req.Header.Get("Authorization") == "" {
			t.Errorf("expected signed request")
		}
		if req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type, got %q", req.Header.Get("Content-Type"))
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("failed to read body: %s", err)
		}
		if !json.Valid(body) {
			t.Errorf("expected JSON body, got %q", string(body))
		}
		if !strings.HasPrefix(string(body), "[") {
			t.Errorf("expected peer site JSON array, got %q", string(body))
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"status":"ok"}`))
	}))
	t.Cleanup(server.Close)

	endpoint, secure, err := normalizeEndpoint(server.URL)
	if err != nil {
		t.Fatalf("unexpected endpoint error: %s", err)
	}

	client := &rustfsClient{
		httpClient: server.Client(),
		endpoint:   endpoint,
		secure:     secure,
		accessKey:  "access",
		secretKey:  "secret",
	}

	_, err = client.SiteReplicationAdd(t.Context(), []peerSite{
		{
			Name:      "site-b",
			Endpoint:  "http://site-b.example.com:9000",
			AccessKey: "access",
			SecretKey: "secret",
		},
	}, srAddOptions{ReplicateILMExpiry: true})
	if err != nil {
		t.Fatalf("unexpected add error: %s", err)
	}
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestRulesListJSONFilterLimitAndTotal(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "rules", "list", "--filter", "PROXY", "--limit", "2", "--json"}, &out)
	if err != nil {
		t.Fatalf("rules list failed: %v", err)
	}
	var got rulesOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Total != 3 || got.Limit != 2 || len(got.Rules) != 2 {
		t.Fatalf("unexpected rules envelope: %+v", got)
	}
	if got.Rules[0].Idx != 1 || got.Rules[0].Type != "DOMAIN-SUFFIX" || got.Rules[0].Payload != "google.com" || got.Rules[0].Proxy != "PROXY" {
		t.Fatalf("unexpected first rule shape: %+v", got.Rules[0])
	}
	if got.Rules[1].Idx != 3 {
		t.Fatalf("rules should sort by idx: %+v", got.Rules)
	}
}

func TestRulesListFilterFieldsAndLimitZero(t *testing.T) {
	srv := fakeMihomo(t, "")
	for _, tt := range []struct {
		name   string
		filter string
		want   string
	}{
		{name: "type", filter: "DOMAIN", want: "DOMAIN-SUFFIX"},
		{name: "payload", filter: "google.com", want: "google.com"},
		{name: "proxy", filter: "FALLBACK", want: "FALLBACK"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run([]string{"--endpoint", srv.URL, "rules", "list", "--filter", tt.filter, "--json"}, &out)
			if err != nil {
				t.Fatalf("rules list filter failed: %v", err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("filtered rules missing %q:\n%s", tt.want, out.String())
			}
		})
	}
	assertCLIError(t, run([]string{"--endpoint", srv.URL, "rules", "list", "--limit", "0"}, &bytes.Buffer{}), exitUsage, "--limit must be > 0")
}

func TestRulesListDefaultLimitAndEmptySchema(t *testing.T) {
	empty := fakeMihomoWith(t, fakeOptions{rules: []map[string]any{}})
	var out bytes.Buffer
	err := run([]string{"--endpoint", empty.URL, "rules", "list", "--json"}, &out)
	if err != nil {
		t.Fatalf("empty rules list failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != `{
  "total": 0,
  "limit": 50,
  "rules": []
}` {
		t.Fatalf("unexpected empty rules JSON:\n%s", out.String())
	}
}

func TestProvidersListJSONShape(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "providers", "list", "--json"}, &out)
	if err != nil {
		t.Fatalf("providers list failed: %v", err)
	}
	var got providersOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Total != 2 || got.Limit != got.Total || len(got.Providers) != 2 {
		t.Fatalf("unexpected providers envelope: %+v", got)
	}
	if got.Providers[0].Name != "airport" || got.Providers[0].Type != "Proxy" || got.Providers[0].VehicleType != "HTTP" || got.Providers[0].Health != "healthy" || got.Providers[0].NodeCount != 2 || got.Providers[0].UpdatedAt != "2026-05-07T04:00:00Z" {
		t.Fatalf("unexpected provider shape: %+v", got.Providers[0])
	}
	if got.Providers[1].Name != "empty" || got.Providers[1].Health != "unknown" || got.Providers[1].NodeCount != 0 {
		t.Fatalf("empty provider should be unknown with zero nodes: %+v", got.Providers[1])
	}
}

func TestProvidersHealthcheckJSONShapeAndTrigger(t *testing.T) {
	seen := ""
	srv := fakeMihomoWith(t, fakeOptions{onHealthcheck: func(provider string) {
		seen = provider
	}})
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "providers", "healthcheck", "airport", "--json"}, &out)
	if err != nil {
		t.Fatalf("providers healthcheck failed: %v", err)
	}
	if seen != "airport" {
		t.Fatalf("healthcheck provider = %q, want airport", seen)
	}
	var got struct {
		Provider    string           `json:"provider"`
		Type        string           `json:"type"`
		VehicleType string           `json:"vehicle_type"`
		Health      string           `json:"health"`
		NodeCount   int              `json:"node_count"`
		UpdatedAt   string           `json:"updated_at"`
		TriggeredAt string           `json:"triggered_at"`
		Results     []map[string]any `json:"results,omitempty"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("invalid raw JSON: %v\n%s", err, out.String())
	}
	if _, ok := raw["results"]; ok || len(raw) != 7 {
		t.Fatalf("healthcheck JSON must contain exactly 7 fields and no results: keys=%v", raw)
	}
	if got.Provider != "airport" || got.Type != "Proxy" || got.VehicleType != "HTTP" || got.Health != "healthy" || got.NodeCount != 2 || got.UpdatedAt != "2026-05-07T04:00:00Z" {
		t.Fatalf("unexpected healthcheck shape: %+v", got)
	}
	if parsed, err := time.Parse(time.RFC3339, got.TriggeredAt); err != nil || parsed.Location() != time.UTC {
		t.Fatalf("triggered_at = %q, parse=%v parsed=%v", got.TriggeredAt, err, parsed)
	}
}

func TestProvidersHealthcheckTriggeredAtAfterSuccessfulTrigger(t *testing.T) {
	times := []time.Time{
		time.Date(2026, 5, 7, 4, 0, 0, 0, time.UTC),
		time.Date(2026, 5, 7, 4, 0, 1, 0, time.UTC),
	}
	calls := 0
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	client, err := newClient(config{endpoint: srv.URL, timeout: defaultTimeout})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	err = runProvidersHealthcheck(context.Background(), &out, config{jsonOut: true}, client, "airport", func() time.Time {
		defer func() { calls++ }()
		return times[calls]
	})
	if err != nil {
		t.Fatalf("healthcheck failed: %v", err)
	}
	if !strings.Contains(out.String(), `"triggered_at": "2026-05-07T04:00:00Z"`) {
		t.Fatalf("triggered_at should be captured from post-trigger clock call:\n%s", out.String())
	}
	if calls != 1 {
		t.Fatalf("now called %d times, want 1", calls)
	}
}

func TestProvidersHealthcheckNotFoundAndEmptyProvider(t *testing.T) {
	srv := fakeMihomo(t, "")
	err := run([]string{"--endpoint", srv.URL, "providers", "healthcheck", "rule-provider"}, &bytes.Buffer{})
	assertCLIError(t, err, exitNotFound, `proxy provider "rule-provider" not found, available: airport, empty`)

	var out bytes.Buffer
	err = run([]string{"--endpoint", srv.URL, "providers", "healthcheck", "empty", "--json"}, &out)
	if err != nil {
		t.Fatalf("empty provider healthcheck failed: %v", err)
	}
	var got providerHealthcheckOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Provider != "empty" || got.Health != "unknown" || got.NodeCount != 0 {
		t.Fatalf("empty provider summary = %+v", got)
	}
}

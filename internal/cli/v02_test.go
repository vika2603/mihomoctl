package cli

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestGroupsDelayJSONShapeAndOrdering(t *testing.T) {
	var seenQuery url.Values
	srv := fakeMihomoWith(t, fakeOptions{
		proxies: map[string]any{
			"Proxy": map[string]any{"name": "Proxy", "type": "Selector", "now": "B", "all": []string{"A", "B", "C"}},
		},
		groupDelays: map[string]map[string]int{"Proxy": {"A": 40, "B": 10}},
		onGroupDelay: func(group string, query url.Values) {
			if group != "Proxy" {
				t.Fatalf("group = %q, want Proxy", group)
			}
			seenQuery = query
		},
	})

	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "groups", "delay", "Proxy", "--delay-timeout", "1500ms", "--url", "https://example.test/generate_204", "--json"}, &out)
	if err != nil {
		t.Fatalf("groups delay --json failed: %v", err)
	}
	if got := seenQuery.Get("timeout"); got != "1500" {
		t.Fatalf("timeout query = %q, want 1500", got)
	}
	if got := seenQuery.Get("url"); got != "https://example.test/generate_204" {
		t.Fatalf("url query = %q", got)
	}

	var got delayOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Group != "Proxy" || got.Type != "Selector" || got.Selected != "B" || got.TestTimeoutMS != 1500 {
		t.Fatalf("unexpected delay envelope: %+v", got)
	}
	if len(got.Results) != 3 {
		t.Fatalf("results len = %d, want 3: %+v", len(got.Results), got.Results)
	}
	if got.Results[0].Node != "B" || got.Results[0].LatencyMS == nil || *got.Results[0].LatencyMS != 10 || got.Results[0].Status != "ok" {
		t.Fatalf("unexpected first result: %+v", got.Results[0])
	}
	if got.Results[1].Node != "A" || got.Results[1].LatencyMS == nil || *got.Results[1].LatencyMS != 40 {
		t.Fatalf("unexpected second result: %+v", got.Results[1])
	}
	if got.Results[2].Node != "C" || got.Results[2].LatencyMS != nil || got.Results[2].Status != "timeout" {
		t.Fatalf("timeout result should be last with null latency: %+v", got.Results[2])
	}
}

func TestGroupsDelaySupportedTypes(t *testing.T) {
	proxies := map[string]any{
		"URL": map[string]any{"name": "URL", "type": "URLTest", "now": "A", "all": []string{"A"}},
		"SEL": map[string]any{"name": "SEL", "type": "Selector", "now": "A", "all": []string{"A"}},
		"FB":  map[string]any{"name": "FB", "type": "Fallback", "now": "A", "all": []string{"A"}},
		"LB":  map[string]any{"name": "LB", "type": "LoadBalance", "now": "A", "all": []string{"A"}},
	}
	delays := map[string]map[string]int{
		"URL": {"A": 1},
		"SEL": {"A": 1},
		"FB":  {"A": 1},
		"LB":  {"A": 1},
	}
	srv := fakeMihomoWith(t, fakeOptions{proxies: proxies, groupDelays: delays})
	for _, group := range []string{"URL", "SEL", "FB", "LB"} {
		t.Run(group, func(t *testing.T) {
			if err := run([]string{"--endpoint", srv.URL, "groups", "delay", group, "--json"}, &bytes.Buffer{}); err != nil {
				t.Fatalf("groups delay %s failed: %v", group, err)
			}
		})
	}
}

func TestGroupsDelayRejectsUnsupportedAndMissingGroups(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{
		proxies: map[string]any{
			"DIRECT": map[string]any{"name": "DIRECT", "type": "Direct"},
			"Reject": map[string]any{"name": "Reject", "type": "Reject"},
			"Proxy":  map[string]any{"name": "Proxy", "type": "Selector", "now": "A", "all": []string{"A"}},
		},
	})
	assertCLIError(t, run([]string{"--endpoint", srv.URL, "groups", "delay", "DIRECT"}, &bytes.Buffer{}), exitUsage, "does not support delay test")
	assertCLIError(t, run([]string{"--endpoint", srv.URL, "groups", "delay", "Reject"}, &bytes.Buffer{}), exitUsage, "does not support delay test")
	assertCLIError(t, run([]string{"--endpoint", srv.URL, "groups", "delay", "Missing"}, &bytes.Buffer{}), exitNotFound, "group \"Missing\" not found")
}

func TestConnectionsListJSONShapeLimitFilterAndOrdering(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "connections", "list", "--limit", "2", "--json"}, &out); err != nil {
		t.Fatalf("connections list --json failed: %v", err)
	}
	var got connectionsOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Total != 3 || got.Limit != 2 || len(got.Connections) != 2 {
		t.Fatalf("unexpected envelope: %+v", got)
	}
	if got.Connections[0].ID != "c-new" || got.Connections[1].ID != "c-mid" {
		t.Fatalf("connections not sorted started_at desc: %+v", got.Connections)
	}
	first := got.Connections[0]
	if first.StartedAt != "2026-05-07T03:00:00Z" || first.Source != "192.0.2.11:52000" || first.Destination != "1.1.1.1:443" || first.Host != "cloudflare.com" || first.Rule != "DOMAIN-SUFFIX" || len(first.Chains) != 2 || first.UploadBytes != 300 || first.DownloadBytes != 400 {
		t.Fatalf("unexpected connection shape: %+v", first)
	}

	for _, tt := range []struct {
		name   string
		filter string
		wantID string
	}{
		{name: "host", filter: "google.com", wantID: "c-old"},
		{name: "source", filter: "198.51.100.9", wantID: "c-mid"},
		{name: "rule", filter: "domain-suffix", wantID: "c-new"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out.Reset()
			err := run([]string{"--endpoint", srv.URL, "connections", "list", "--filter", tt.filter, "--json"}, &out)
			if err != nil {
				t.Fatalf("connections filter failed: %v", err)
			}
			var filtered connectionsOutput
			if err := json.Unmarshal(out.Bytes(), &filtered); err != nil {
				t.Fatalf("invalid JSON: %v\n%s", err, out.String())
			}
			if filtered.Total != 1 || len(filtered.Connections) != 1 || filtered.Connections[0].ID != tt.wantID {
				t.Fatalf("filter %q result = %+v, want %s", tt.filter, filtered, tt.wantID)
			}
		})
	}
}

func TestConnectionsListLimitBoundariesAndEmptyJSON(t *testing.T) {
	srv := fakeMihomo(t, "")
	assertCLIError(t, run([]string{"--endpoint", srv.URL, "connections", "list", "--limit", "0"}, &bytes.Buffer{}), exitUsage, "--limit must be > 0")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "connections", "list", "--limit", "99", "--json"}, &out); err != nil {
		t.Fatalf("connections list large limit failed: %v", err)
	}
	var got connectionsOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Total != 3 || got.Limit != 99 || len(got.Connections) != 3 {
		t.Fatalf("large limit should return all connections: %+v", got)
	}

	empty := fakeMihomoWith(t, fakeOptions{connections: []map[string]any{}})
	out.Reset()
	if err := run([]string{"--endpoint", empty.URL, "connections", "list", "--json"}, &out); err != nil {
		t.Fatalf("empty connections list failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != `{
  "total": 0,
  "limit": 50,
  "connections": []
}` {
		t.Fatalf("unexpected empty JSON:\n%s", out.String())
	}
}

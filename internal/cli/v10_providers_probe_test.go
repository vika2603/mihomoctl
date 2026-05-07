package cli

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestProxyProvidersListAndGet(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "list", "--json"}, &out); err != nil {
		t.Fatalf("proxy-providers list --json failed: %v", err)
	}
	var list providersOutput
	if err := json.Unmarshal(out.Bytes(), &list); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if list.Total != 2 || len(list.Providers) != 2 {
		t.Fatalf("unexpected provider list: %+v", list)
	}
	if list.Providers[0].Name != "airport" || list.Providers[0].Type != "Proxy" || list.Providers[0].Health != "healthy" || list.Providers[0].NodeCount != 2 {
		t.Fatalf("unexpected first provider: %+v", list.Providers[0])
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "get", "airport"}, &out); err != nil {
		t.Fatalf("proxy-providers get failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"airport (HTTP)", "health: healthy", "nodes: 2", "updated_at: 2026-05-07T04:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("proxy-providers get output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "get", "airport", "--json"}, &out); err != nil {
		t.Fatalf("proxy-providers get --json failed: %v", err)
	}
	var one providerOutput
	if err := json.Unmarshal(out.Bytes(), &one); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if one.Name != "airport" || one.VehicleType != "HTTP" || one.NodeCount != 2 {
		t.Fatalf("unexpected provider JSON: %+v", one)
	}
}

func TestRuleProvidersList(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "rule-providers", "list"}, &out); err != nil {
		t.Fatalf("rule-providers list failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"name\ttype\tvehicle_type\tbehavior\trule_count\tupdated_at", "geoip\tRule\tHTTP\tIPCIDR\t1024\t2026-05-07T05:00:00Z", "rejects\tRule\tFile\tDomain\t3\t2026-05-07T04:30:00Z"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rule-providers list output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "rule-providers", "list", "--json"}, &out); err != nil {
		t.Fatalf("rule-providers list --json failed: %v", err)
	}
	var jsonGot ruleProvidersOutput
	if err := json.Unmarshal(out.Bytes(), &jsonGot); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if jsonGot.Total != 2 || len(jsonGot.Providers) != 2 {
		t.Fatalf("unexpected rule providers: %+v", jsonGot)
	}
	if jsonGot.Providers[0].Name != "geoip" || jsonGot.Providers[0].Behavior != "IPCIDR" || jsonGot.Providers[0].RuleCount != 1024 {
		t.Fatalf("unexpected first rule provider: %+v", jsonGot.Providers[0])
	}
}

func TestProxyProvidersHealthcheckAndSafetyFlags(t *testing.T) {
	seen := ""
	srv := fakeMihomoWith(t, fakeOptions{onHealthcheck: func(provider string) {
		seen = provider
	}})

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "healthcheck", "airport", "--yes", "--json"}, &out); err != nil {
		t.Fatalf("proxy-providers healthcheck --yes --json failed: %v", err)
	}
	if seen != "airport" {
		t.Fatalf("healthcheck provider = %q, want airport", seen)
	}
	var got providerHealthcheckOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("invalid raw JSON: %v\n%s", err, out.String())
	}
	if len(raw) != 7 {
		t.Fatalf("healthcheck JSON must retain exactly 7 fields; keys=%v", raw)
	}
	if _, ok := raw["risk"]; ok {
		t.Fatalf("healthcheck is side-effecting read and must not grow risk field: keys=%v", raw)
	}
	if got.Provider != "airport" || got.Health != "healthy" || got.NodeCount != 2 {
		t.Fatalf("unexpected healthcheck output: %+v", got)
	}

	err := run([]string{"--endpoint", srv.URL, "proxy-providers", "healthcheck", "airport", "--dry-run"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, "does not support --dry-run")
	assertCLIError(t, err, exitUsage, "Drop --dry-run to run the healthcheck")
}

func TestProxyProvidersUpdateMutationSafety(t *testing.T) {
	updateCount := 0
	srv := fakeMihomoWith(t, fakeOptions{onProviderUpdate: func(provider string) {
		if provider != "airport" {
			t.Fatalf("update provider = %q, want airport", provider)
		}
		updateCount++
	}})

	err := run([]string{"--endpoint", srv.URL, "proxy-providers", "update", "airport"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, "non-interactive session requires --yes")
	if updateCount != 0 {
		t.Fatalf("non-TTY abort still called update endpoint %d times", updateCount)
	}

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "update", "airport", "--dry-run", "--json"}, &out); err != nil {
		t.Fatalf("proxy-providers update --dry-run --json failed: %v", err)
	}
	if updateCount != 0 {
		t.Fatalf("dry-run called update endpoint %d times", updateCount)
	}
	var dry providerUpdateOutput
	if err := json.Unmarshal(out.Bytes(), &dry); err != nil {
		t.Fatalf("invalid dry-run JSON: %v\n%s", err, out.String())
	}
	if dry.Provider != "airport" || dry.Updated || !dry.DryRun || dry.DryRunMode != "client_simulated" {
		t.Fatalf("unexpected dry-run output: %+v", dry)
	}
	if dry.Risk == nil || dry.Risk.Level != "medium" {
		t.Fatalf("dry-run risk = %+v, want medium", dry.Risk)
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "update", "airport", "--yes", "--json"}, &out); err != nil {
		t.Fatalf("proxy-providers update --yes --json failed: %v", err)
	}
	if updateCount != 1 {
		t.Fatalf("update endpoint called %d times, want 1", updateCount)
	}
	var updated providerUpdateOutput
	if err := json.Unmarshal(out.Bytes(), &updated); err != nil {
		t.Fatalf("invalid update JSON: %v\n%s", err, out.String())
	}
	if updated.Provider != "airport" || !updated.Updated || updated.DryRun || updated.DryRunMode != "" {
		t.Fatalf("unexpected update output: %+v", updated)
	}
	if updated.Risk == nil || updated.Risk.Level != "medium" {
		t.Fatalf("update risk = %+v, want medium", updated.Risk)
	}
}

func TestProxyDelayJSONAndQuery(t *testing.T) {
	var seen url.Values
	srv := fakeMihomoWith(t, fakeOptions{
		proxyDelays: map[string]int{"Auto / 香港": 33},
		onProxyDelay: func(proxy string, query url.Values) {
			if proxy != "Auto / 香港" {
				t.Fatalf("proxy = %q, want Auto / 香港", proxy)
			}
			seen = query
		},
	})

	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "proxy", "delay", "Auto / 香港", "--delay-timeout", "1500ms", "--url", "https://example.test/generate_204", "--expected", "204-299", "--json"}, &out)
	if err != nil {
		t.Fatalf("proxy delay --json failed: %v", err)
	}
	if got := seen.Get("timeout"); got != "1500" {
		t.Fatalf("timeout query = %q, want 1500", got)
	}
	if got := seen.Get("url"); got != "https://example.test/generate_204" {
		t.Fatalf("url query = %q", got)
	}
	if got := seen.Get("expected"); got != "204-299" {
		t.Fatalf("expected query = %q", got)
	}

	var got proxyDelayOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Proxy != "Auto / 香港" || got.DelayMS != 33 || got.TestTimeoutMS != 1500 || got.ExpectedStatus != "204-299" {
		t.Fatalf("unexpected proxy delay output: %+v", got)
	}
}

func TestProvidersProbeUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bare proxy-providers", args: []string{"proxy-providers"}, want: "proxy-providers requires list, get, update, or healthcheck"},
		{name: "proxy-providers get missing", args: []string{"proxy-providers", "get"}, want: "proxy-providers get requires <name>"},
		{name: "old providers namespace removed", args: []string{"providers", "list"}, want: `unknown command "providers"`},
		{name: "old providers healthcheck removed", args: []string{"providers", "healthcheck", "airport"}, want: `unknown command "providers"`},
		{name: "bare rule-providers", args: []string{"rule-providers"}, want: "rule-providers requires list"},
		{name: "rule-providers unknown", args: []string{"rule-providers", "get", "x"}, want: `unknown rule-providers subcommand "get"`},
		{name: "proxy delay missing", args: []string{"proxy", "delay"}, want: "proxy delay requires <node>"},
		{name: "proxy delay bad timeout", args: []string{"proxy", "delay", "A", "--delay-timeout", "0s"}, want: "--delay-timeout must be > 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args, &bytes.Buffer{})
			assertCLIError(t, err, exitUsage, tt.want)
		})
	}
}

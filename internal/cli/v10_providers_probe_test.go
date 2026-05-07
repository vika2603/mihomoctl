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
		{name: "bare proxy-providers", args: []string{"proxy-providers"}, want: "proxy-providers requires list or get"},
		{name: "proxy-providers get missing", args: []string{"proxy-providers", "get"}, want: "proxy-providers get requires <name>"},
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

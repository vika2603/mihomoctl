package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestStatus(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "status"}, &out)
	if err != nil {
		t.Fatalf("status failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"mode: rule", "version: v-test", "groups: 2 selectable"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
}

func TestStatusDefaultSummary(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "status"}, &out); err != nil {
		t.Fatalf("status failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"mode: rule", "version: v-test", "groups: 2 selectable (use --verbose to list)"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status output missing %q:\n%s", want, got)
		}
	}
	for _, unexpected := range []string{"Proxy: A", "Auto / 香港: A"} {
		if strings.Contains(got, unexpected) {
			t.Fatalf("status default summary should not list group %q:\n%s", unexpected, got)
		}
	}
}

func TestStatusVerboseListsGroups(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "status", "--verbose"}, &out); err != nil {
		t.Fatalf("status --verbose failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"mode: rule", "version: v-test", "groups:", "Proxy: A", "Auto / 香港: A"} {
		if !strings.Contains(got, want) {
			t.Fatalf("status --verbose output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "(use --verbose to list)") {
		t.Fatalf("status --verbose should not print summary hint:\n%s", got)
	}
}

func TestStatusJSON(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "status", "--json"}, &out)
	if err != nil {
		t.Fatalf("status --json failed: %v", err)
	}
	var got struct {
		Mode    string        `json:"mode"`
		Version string        `json:"version"`
		Groups  []groupOutput `json:"groups"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Mode != "rule" || got.Version != "v-test" || len(got.Groups) != 2 || got.Groups[0].Selected != "A" {
		t.Fatalf("unexpected JSON: %+v", got)
	}
	if got.Groups[0].Name == "" || got.Groups[0].Type == "" || len(got.Groups[0].Candidates) == 0 {
		t.Fatalf("status JSON group shape incomplete: %+v", got.Groups[0])
	}
}

func TestPostCommandGlobalFlags(t *testing.T) {
	srv := fakeMihomo(t, "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "proxy list json",
			args: []string{"proxy", "list", "--endpoint", srv.URL, "--json"},
			want: `"candidates"`,
		},
		{
			name: "proxy set endpoint",
			args: []string{"proxy", "set", "--endpoint", srv.URL, "Proxy", "B"},
			want: "Proxy: B (was A)",
		},
		{
			name: "mode get json",
			args: []string{"mode", "get", "--endpoint", srv.URL, "--json"},
			want: `"mode"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tt.args, &out); err != nil {
				t.Fatalf("run(%v) failed: %v", tt.args, err)
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("output missing %q:\n%s", tt.want, out.String())
			}
		})
	}
}

func TestProxyListAndSet(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy", "list"}, &out); err != nil {
		t.Fatalf("proxy list failed: %v", err)
	}
	if !strings.HasPrefix(out.String(), "name\ttype\tselected\tcandidates\n") {
		t.Fatalf("proxy list default should render summary table:\n%s", out.String())
	}
	if strings.Contains(out.String(), "* A") || strings.Contains(out.String(), "  B") {
		t.Fatalf("proxy list default should not render candidate nodes:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "proxy", "list", "--verbose"}, &out); err != nil {
		t.Fatalf("proxy list --verbose failed: %v", err)
	}
	if !strings.Contains(out.String(), "Proxy -> A") || !strings.Contains(out.String(), "* A") {
		t.Fatalf("unexpected proxy list --verbose:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "proxy", "set", "Proxy", "B"}, &out); err != nil {
		t.Fatalf("proxy set failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "Proxy: B (was A)" {
		t.Fatalf("unexpected proxy set output: %q", out.String())
	}
}

func TestProxySetEscapesGroupPath(t *testing.T) {
	seenRequestURI := ""
	srv := fakeMihomoWith(t, fakeOptions{
		proxies: map[string]any{
			"Auto / 香港": map[string]any{"name": "Auto / 香港", "type": "Selector", "now": "A", "all": []string{"A", "B"}},
		},
		onProxySet: func(uri string) {
			seenRequestURI = uri
		},
	})
	defer srv.Close()

	var out bytes.Buffer
	group := "Auto / 香港"
	err := run([]string{"--endpoint", srv.URL, "proxy", "set", group, "B"}, &out)
	if err != nil {
		t.Fatalf("proxy set with escaped group failed: %v", err)
	}
	wantURI := "/proxies/Auto%20%2F%20%E9%A6%99%E6%B8%AF"
	if seenRequestURI != wantURI {
		t.Fatalf("request URI = %q, want %q", seenRequestURI, wantURI)
	}
	if strings.TrimSpace(out.String()) != group+": B (was A)" {
		t.Fatalf("unexpected proxy set output: %q", out.String())
	}
}

func TestModeGetAndSet(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "mode", "get"}, &out); err != nil {
		t.Fatalf("mode get failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "rule" {
		t.Fatalf("unexpected mode get output: %q", out.String())
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "mode", "set", "direct"}, &out); err != nil {
		t.Fatalf("mode set failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "mode: direct (was rule)" {
		t.Fatalf("unexpected mode set output: %q", out.String())
	}
}

func TestSubcommandHelp(t *testing.T) {
	for _, args := range [][]string{{"proxy", "--help"}, {"mode", "--help"}} {
		var out bytes.Buffer
		if err := run(args, &out); err != nil {
			t.Fatalf("run(%v) failed: %v", args, err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("help output missing usage for %v: %q", args, out.String())
		}
	}
}

func TestCompletionCommandDisabled(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--help"}, &out); err != nil {
		t.Fatalf("root help failed: %v", err)
	}
	if strings.Contains(out.String(), "completion") {
		t.Fatalf("root help exposes completion command:\n%s", out.String())
	}

	out.Reset()
	err := run([]string{"completion", "--help"}, &out)
	assertCLIError(t, err, exitUsage, `unknown command "completion"`)
}

func TestRootUnknownCommandSuggestsNearestCommand(t *testing.T) {
	err := run([]string{"sttaus"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, `unknown command "sttaus"`)
	assertCLIError(t, err, exitUsage, "Did you mean this?")
	assertCLIError(t, err, exitUsage, "status")
}

func TestNotFoundExitCode(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "proxy", "set", "Missing", "B"}, &out)
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected cliError, got %T %[1]v", err)
	}
	if ce.Code != exitNotFound {
		t.Fatalf("exit code = %d, want %d", ce.Code, exitNotFound)
	}
}

func TestAuthError(t *testing.T) {
	srv := fakeMihomo(t, "secret")
	var out bytes.Buffer
	err := run([]string{"--endpoint", srv.URL, "status"}, &out)
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected cliError, got %T %[1]v", err)
	}
	if ce.Code != exitNoPerm || !strings.Contains(ce.Message, "MIHOMOCTL_SECRET") {
		t.Fatalf("unexpected auth error: code=%d msg=%q", ce.Code, ce.Message)
	}
}

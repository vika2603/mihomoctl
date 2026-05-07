package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestGroupsListColumnsSelection asserts --columns filters and reorders the
// tab-separated header and body rows on non-TTY output (--json is unchanged).
func TestGroupsListColumnsSelection(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "groups", "list", "--columns", "name,selected"}, &out); err != nil {
		t.Fatalf("groups list --columns name,selected failed: %v", err)
	}
	got := out.String()
	wantHeader := "name\tselected\n"
	if !strings.HasPrefix(got, wantHeader) {
		t.Fatalf("header = %q, want %q prefix", got, wantHeader)
	}
	if strings.Contains(got, "type") || strings.Contains(got, "candidates") {
		t.Fatalf("excluded columns leaked into output:\n%s", got)
	}

	// Reordered selection
	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "groups", "list", "--columns", "candidates,name"}, &out); err != nil {
		t.Fatalf("groups list --columns candidates,name failed: %v", err)
	}
	if !strings.HasPrefix(out.String(), "candidates\tname\n") {
		t.Fatalf("reordered header missing: %s", out.String())
	}
}

// TestGroupsListColumnsRejectsUnknownLister asserts an unknown column name
// produces exit 64 with a usage error that lists every available column.
func TestGroupsListColumnsRejectsUnknown(t *testing.T) {
	srv := fakeMihomo(t, "")

	err := run([]string{"--endpoint", srv.URL, "groups", "list", "--columns", "bogus"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, `unknown column "bogus"`)
	assertCLIError(t, err, exitUsage, "available: name,type,selected,candidates")
}

// TestConnectionsListColumnsSelection asserts the same flag works on
// connections list including the synthetic combined `up_down` column.
func TestConnectionsListColumnsSelection(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "connections", "list", "--columns", "destination,up_down"}, &out); err != nil {
		t.Fatalf("connections list --columns failed: %v", err)
	}
	got := out.String()
	if !strings.HasPrefix(got, "destination\tup/down\n") {
		t.Fatalf("connections list --columns header = %q", got)
	}
	if strings.Contains(got, "started_at") || strings.Contains(got, "rule") {
		t.Fatalf("excluded columns leaked:\n%s", got)
	}
}

// TestProxyProvidersListColumnsSelection sanity check.
func TestProxyProvidersListColumnsSelection(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy-providers", "list", "--columns", "name,health"}, &out); err != nil {
		t.Fatalf("proxy-providers list --columns failed: %v", err)
	}
	if !strings.HasPrefix(out.String(), "name\thealth\n") {
		t.Fatalf("proxy-providers header = %q", out.String())
	}
}

// TestRuleProvidersListColumnsSelection sanity check.
func TestRuleProvidersListColumnsSelection(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "rule-providers", "list", "--columns", "name,rule_count"}, &out); err != nil {
		t.Fatalf("rule-providers list --columns failed: %v", err)
	}
	if !strings.HasPrefix(out.String(), "name\trule_count\n") {
		t.Fatalf("rule-providers header = %q", out.String())
	}
}

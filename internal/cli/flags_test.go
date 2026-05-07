package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitGlobalFlagsValueForms(t *testing.T) {
	srv := fakeMihomo(t, "")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "endpoint equals",
			args: []string{"status", "--endpoint=" + srv.URL},
			want: "mode:",
		},
		{
			name: "secret equals",
			args: []string{"--endpoint", srv.URL, "status", "--secret=secret"},
			want: "mode:",
		},
		{
			name: "secret shorthand",
			args: []string{"-s", "secret", "--endpoint", srv.URL, "status"},
			want: "mode:",
		},
		{
			name: "json in command middle",
			args: []string{"--endpoint", srv.URL, "proxy", "--json", "list"},
			want: `"groups"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := run(tt.args, &out); err != nil {
				t.Fatalf("run(%v) failed: %v", tt.args, err)
			}
			if !bytes.Contains(out.Bytes(), []byte(tt.want)) {
				t.Fatalf("output missing %q:\n%s", tt.want, out.String())
			}
		})
	}
}

func TestSplitGlobalFlagsMissingValue(t *testing.T) {
	for _, args := range [][]string{
		{"--endpoint"},
		{"status", "--secret"},
		{"proxy", "list", "--timeout"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			err := run(args, &bytes.Buffer{})
			assertCLIError(t, err, exitUsage, "flag needs an argument")
		})
	}
}

func TestSecretEnvDoesNotLeakInHelp(t *testing.T) {
	t.Setenv("MIHOMOCTL_SECRET", "test_secret_value")
	argsList := secretHelpSurfaces()
	if len(argsList) != 29 {
		t.Fatalf("secret help surface count = %d, want 29", len(argsList))
	}
}

func TestSecretEnvDoesNotLeakInImplementedHelp(t *testing.T) {
	t.Setenv("MIHOMOCTL_SECRET", "test_secret_value")
	for _, args := range secretHelpSurfaces() {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out bytes.Buffer
			if err := run(args, &out); err != nil {
				t.Fatalf("run(%v) failed: %v", args, err)
			}
			if strings.Contains(out.String(), "test_secret_value") {
				t.Fatalf("help output leaked MIHOMOCTL_SECRET:\n%s", out.String())
			}
		})
	}
}

func secretHelpSurfaces() [][]string {
	return [][]string{
		{"--help"},
		{"system", "--help"},
		{"system", "ping", "--help"},
		{"system", "version", "--help"},
		{"groups", "--help"},
		{"groups", "list", "--help"},
		{"groups", "get", "--help"},
		{"groups", "delay", "--help"},
		{"status", "--help"},
		{"connections", "list", "--help"},
		{"proxy", "set", "--help"},
		{"mode", "set", "--help"},
		{"rules", "--help"},
		{"rules", "list", "--help"},
		{"providers", "--help"},
		{"providers", "list", "--help"},
		{"providers", "healthcheck", "--help"},
		{"connections", "--help"},
		{"connections", "watch", "--help"},
		{"conns", "--help"},
		{"conns", "list", "--help"},
		{"conns", "watch", "--help"},
		{"dns", "--help"},
		{"dns", "query", "--help"},
		{"cache", "--help"},
		{"cache", "clear", "--help"},
		{"cache", "clear", "fakeip", "--help"},
		{"cache", "clear", "dns", "--help"},
		{"cache", "clear", "all", "--help"},
	}
}

func TestSecretEnvUsedAtExecution(t *testing.T) {
	t.Setenv("MIHOMOCTL_SECRET", "secret")
	srv := fakeMihomo(t, "secret")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "status"}, &out); err != nil {
		t.Fatalf("status with MIHOMOCTL_SECRET failed: %v", err)
	}
	if !strings.Contains(out.String(), "mode:") {
		t.Fatalf("unexpected status output:\n%s", out.String())
	}
}

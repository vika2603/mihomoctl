package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSystemVersionHumanAndJSON(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "system", "version"}, &out); err != nil {
		t.Fatalf("system version failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "v-test" {
		t.Fatalf("unexpected system version output: %q", out.String())
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "--json", "system", "version"}, &out); err != nil {
		t.Fatalf("system version --json failed: %v", err)
	}
	var got systemVersionOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Version != "v-test" {
		t.Fatalf("unexpected JSON: %+v", got)
	}
}

func TestSystemPingHumanAndJSON(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "system", "ping"}, &out); err != nil {
		t.Fatalf("system ping failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"ok: true", "controller: " + srv.URL, "version: v-test"} {
		if !strings.Contains(got, want) {
			t.Fatalf("system ping output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "system", "ping", "--json"}, &out); err != nil {
		t.Fatalf("system ping --json failed: %v", err)
	}
	var jsonGot systemPingOutput
	if err := json.Unmarshal(out.Bytes(), &jsonGot); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if !jsonGot.OK || jsonGot.Controller != srv.URL || jsonGot.Version != "v-test" {
		t.Fatalf("unexpected JSON: %+v", jsonGot)
	}
}

func TestSystemCommandUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bare system", args: []string{"system"}, want: "system requires ping or version"},
		{name: "unknown", args: []string{"system", "versoin"}, want: "Did you mean this?"},
		{name: "ping args", args: []string{"system", "ping", "extra"}, want: "system ping takes no arguments"},
		{name: "version args", args: []string{"system", "version", "extra"}, want: "system version takes no arguments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args, &bytes.Buffer{})
			assertCLIError(t, err, exitUsage, tt.want)
		})
	}
}

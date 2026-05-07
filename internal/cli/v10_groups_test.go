package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestGroupsListHumanAndJSON(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "groups", "list"}, &out); err != nil {
		t.Fatalf("groups list failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"name\ttype\tselected\tcandidates", "Auto / 香港\tSelector\tA\t2", "Proxy\tSelector\tA\t2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("groups list output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "\nA\t") {
		t.Fatalf("groups list should not include leaf proxies:\n%s", got)
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "groups", "list", "--json"}, &out); err != nil {
		t.Fatalf("groups list --json failed: %v", err)
	}
	var jsonGot groupsOutput
	if err := json.Unmarshal(out.Bytes(), &jsonGot); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if jsonGot.Total != 2 || len(jsonGot.Groups) != 2 {
		t.Fatalf("unexpected groups list JSON: %+v", jsonGot)
	}
	if jsonGot.Groups[0].Name != "Auto / 香港" || jsonGot.Groups[1].Name != "Proxy" {
		t.Fatalf("groups not sorted by name: %+v", jsonGot.Groups)
	}
	if jsonGot.Groups[1].Selected != "A" || len(jsonGot.Groups[1].Candidates) != 2 {
		t.Fatalf("unexpected group JSON row: %+v", jsonGot.Groups[1])
	}
}

func TestGroupsGetHumanAndJSON(t *testing.T) {
	srv := fakeMihomo(t, "")

	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "groups", "get", "Proxy"}, &out); err != nil {
		t.Fatalf("groups get failed: %v", err)
	}
	got := out.String()
	for _, want := range []string{"Proxy (Selector)", "selected: A", "* A", "  B"} {
		if !strings.Contains(got, want) {
			t.Fatalf("groups get output missing %q:\n%s", want, got)
		}
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "--json", "groups", "get", "Proxy"}, &out); err != nil {
		t.Fatalf("groups get --json failed: %v", err)
	}
	var jsonGot groupOutput
	if err := json.Unmarshal(out.Bytes(), &jsonGot); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if jsonGot.Name != "Proxy" || jsonGot.Type != "Selector" || jsonGot.Selected != "A" || len(jsonGot.Candidates) != 2 {
		t.Fatalf("unexpected groups get JSON: %+v", jsonGot)
	}
}

func TestGroupsGetEscapesNameAndNotFound(t *testing.T) {
	srv := fakeMihomo(t, "")

	if err := run([]string{"--endpoint", srv.URL, "groups", "get", "Auto / 香港"}, &bytes.Buffer{}); err != nil {
		t.Fatalf("groups get should escape group path: %v", err)
	}

	err := run([]string{"--endpoint", srv.URL, "groups", "get", "Missing"}, &bytes.Buffer{})
	assertCLIError(t, err, exitNotFound, "mihomo endpoint or requested resource not found")
}

func TestGroupsCommandUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "bare groups", args: []string{"groups"}, want: "groups requires list or get"},
		{name: "unknown", args: []string{"groups", "bad"}, want: `unknown groups subcommand "bad"`},
		{name: "list args", args: []string{"groups", "list", "extra"}, want: "groups list takes no arguments"},
		{name: "get args", args: []string{"groups", "get"}, want: "groups get requires <name>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := run(tt.args, &bytes.Buffer{})
			assertCLIError(t, err, exitUsage, tt.want)
		})
	}
}

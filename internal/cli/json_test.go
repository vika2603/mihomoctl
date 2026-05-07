package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestProxyListJSONShape(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy", "list", "--json"}, &out); err != nil {
		t.Fatalf("proxy list --json failed: %v", err)
	}
	var got struct {
		Groups []groupOutput `json:"groups"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if len(got.Groups) != 2 {
		t.Fatalf("groups len = %d, want 2: %+v", len(got.Groups), got.Groups)
	}
	if got.Groups[0].Name != "Auto / 香港" || got.Groups[0].Type != "Selector" || got.Groups[0].Selected != "A" || len(got.Groups[0].Candidates) != 2 {
		t.Fatalf("unexpected group shape: %+v", got.Groups[0])
	}
}

func TestProxySetJSONShape(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "proxy", "set", "Proxy", "B", "--json"}, &out); err != nil {
		t.Fatalf("proxy set --json failed: %v", err)
	}
	var got struct {
		Group    string `json:"group"`
		Selected string `json:"selected"`
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Group != "Proxy" || got.Selected != "B" || got.Previous != "A" {
		t.Fatalf("unexpected proxy set JSON: %+v", got)
	}
}

func TestModeGetJSONShape(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "mode", "get", "--json"}, &out); err != nil {
		t.Fatalf("mode get --json failed: %v", err)
	}
	var got struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Mode != "rule" {
		t.Fatalf("unexpected mode get JSON: %+v", got)
	}
}

func TestModeSetJSONShape(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "mode", "set", "direct", "--json"}, &out); err != nil {
		t.Fatalf("mode set --json failed: %v", err)
	}
	var got struct {
		Mode     string `json:"mode"`
		Previous string `json:"previous"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Mode != "direct" || got.Previous != "rule" {
		t.Fatalf("unexpected mode set JSON: %+v", got)
	}
}

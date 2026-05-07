package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseColumns(t *testing.T) {
	tests := map[string][]string{
		"":               nil,
		"name":           {"name"},
		"name,type":      {"name", "type"},
		" name , type ":  {"name", "type"},
		"name,,type":     {"name", "type"},
		",":              nil,
		"name,type,name": {"name", "type", "name"},
	}
	for in, want := range tests {
		got := ParseColumns(in)
		if len(got) != len(want) {
			t.Fatalf("ParseColumns(%q) = %v, want %v", in, got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("ParseColumns(%q)[%d] = %q, want %q", in, i, got[i], want[i])
			}
		}
	}
}

func TestTableSpecSelectAll(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}
	got, err := spec.Select(nil)
	if err != nil {
		t.Fatalf("Select(nil) err = %v", err)
	}
	if len(got) != 3 || got[0].Name != "a" || got[2].Name != "c" {
		t.Fatalf("Select(nil) = %+v, want all 3 cols in order", got)
	}
}

func TestTableSpecSelectDefault(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
		Default: []string{"a", "c"},
	}
	got, err := spec.Select(nil)
	if err != nil {
		t.Fatalf("Select(nil) err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "c" {
		t.Fatalf("Select(nil) = %+v, want default [a, c]", got)
	}
}

func TestTableSpecSelectExplicit(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "a"}, {Name: "b"}, {Name: "c"}},
	}
	got, err := spec.Select([]string{"c", "a"})
	if err != nil {
		t.Fatalf("Select err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "c" || got[1].Name != "a" {
		t.Fatalf("Select preserves caller order: got %+v, want [c, a]", got)
	}
}

func TestTableSpecSelectDedupes(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "a"}, {Name: "b"}},
	}
	got, err := spec.Select([]string{"a", "b", "a"})
	if err != nil {
		t.Fatalf("Select err = %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("Select dedupes: got %+v, want [a, b]", got)
	}
}

func TestTableSpecSelectUnknownColumnListsAvailable(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "name"}, {Name: "type"}},
	}
	_, err := spec.Select([]string{"bogus"})
	if err == nil {
		t.Fatal("Select(bogus) err = nil, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "unknown column \"bogus\"") {
		t.Fatalf("error message missing 'unknown column \"bogus\"': %s", msg)
	}
	if !strings.Contains(msg, "available: name,type") {
		t.Fatalf("error message missing available list: %s", msg)
	}
}

func TestTableSpecSelectEmptyAfterTrim(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "a"}},
	}
	_, err := spec.Select([]string{"  ", "  "})
	if err == nil {
		t.Fatal("Select(blanks) err = nil, want error")
	}
}

func TestTableSpecAvailableNames(t *testing.T) {
	spec := TableSpec{
		Columns: []Column{{Name: "name"}, {Name: "type"}, {Name: "selected"}},
	}
	if got := spec.AvailableNames(); got != "name,type,selected" {
		t.Fatalf("AvailableNames = %q, want name,type,selected", got)
	}
}

func TestWriteTableNonTTYIsTabSeparated(t *testing.T) {
	cols := []Column{{Name: "name"}, {Name: "type"}}
	rows := [][]string{
		{"Auto / 香港", "URLTest"},
		{"Proxy", "Selector"},
	}
	var buf bytes.Buffer
	if err := WriteTable(&buf, cols, rows); err != nil {
		t.Fatalf("WriteTable err = %v", err)
	}
	got := buf.String()
	want := "name\ttype\nAuto / 香港\tURLTest\nProxy\tSelector\n"
	if got != want {
		t.Fatalf("WriteTable non-TTY:\n got %q\nwant %q", got, want)
	}
}

func TestWriteTableHeaderOverride(t *testing.T) {
	cols := []Column{{Name: "up_down", Header: "up/down"}}
	rows := [][]string{{"1B/2B"}}
	var buf bytes.Buffer
	if err := WriteTable(&buf, cols, rows); err != nil {
		t.Fatalf("WriteTable err = %v", err)
	}
	if got := buf.String(); got != "up/down\n1B/2B\n" {
		t.Fatalf("WriteTable header override = %q, want %q", got, "up/down\n1B/2B\n")
	}
}

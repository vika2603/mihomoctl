package render

import (
	"strings"
	"testing"
)

func TestFormatBytesIEC(t *testing.T) {
	tests := map[int64]string{
		0:          "0 B",
		123:        "123 B",
		1024:       "1.0 KiB",
		1536:       "1.5 KiB",
		1048576:    "1.0 MiB",
		1073741824: "1.0 GiB",
		-1536:      "-1.5 KiB",
	}
	for n, want := range tests {
		if got := FormatBytes(n); got != want {
			t.Fatalf("FormatBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestHumanTableUsesHeadersAndRows(t *testing.T) {
	got := HumanTable([]string{"name", "up/down"}, [][]string{{"c1", "1.0 KiB/2.0 KiB"}}, 0)
	for _, want := range []string{"name", "up/down", "c1", "1.0 KiB/2.0 KiB"} {
		if !strings.Contains(got, want) {
			t.Fatalf("table missing %q:\n%s", want, got)
		}
	}
}

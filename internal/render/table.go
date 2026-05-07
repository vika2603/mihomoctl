package render

import (
	"fmt"
	"io"
	"strings"
)

// Column describes one column in a human-readable table.
//
// Name is the user-facing key used by the --columns flag for selection;
// it MUST match the JSON field name on the corresponding output struct.
// Header is the rendered header text (defaults to Name when empty).
type Column struct {
	Name   string
	Header string
}

// TableSpec describes the full set of columns a command can render plus
// a default subset shown when --columns is not supplied.
type TableSpec struct {
	// Columns is the full ordered set of available columns.
	Columns []Column
	// Default is the subset (by Name) shown when --columns is empty.
	// If Default is empty, all columns in Columns are shown.
	Default []string
}

// AvailableNames returns the comma-separated list of column names a
// caller can pass to --columns. The order matches Columns.
func (s TableSpec) AvailableNames() string {
	names := make([]string, len(s.Columns))
	for i, c := range s.Columns {
		names[i] = c.Name
	}
	return strings.Join(names, ",")
}

// Select resolves the user-provided --columns input into the concrete
// ordered Column slice to render. names == nil means "use Default
// (or full set if Default is empty)". An unknown name returns an
// error whose message lists every available column.
func (s TableSpec) Select(names []string) ([]Column, error) {
	if len(names) == 0 {
		if len(s.Default) > 0 {
			names = s.Default
		} else {
			out := make([]Column, len(s.Columns))
			copy(out, s.Columns)
			return out, nil
		}
	}
	byName := make(map[string]Column, len(s.Columns))
	for _, c := range s.Columns {
		byName[c.Name] = c
	}
	out := make([]Column, 0, len(names))
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		c, ok := byName[n]
		if !ok {
			return nil, fmt.Errorf("unknown column %q; available: %s", n, s.AvailableNames())
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no columns selected; available: %s", s.AvailableNames())
	}
	return out, nil
}

// ParseColumns parses a comma-separated --columns flag value into
// individual column names. Whitespace around each name is trimmed and
// empty entries are dropped. Returns nil for empty input.
func ParseColumns(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// WriteTable renders rows to w using cols. On an interactive terminal
// the output uses the lipgloss bordered table. On a non-TTY writer
// (pipe / file / redirected stdout) the output is tab-separated for
// stable scripting (header line followed by one tab-separated row per
// rows entry). The non-TTY path is byte-stable for grep/awk pipelines.
//
// rows is a row-major matrix; len(rows[i]) MUST equal len(cols).
func WriteTable(w io.Writer, cols []Column, rows [][]string) error {
	if SupportsInteractiveTerminal(w) {
		return writeCharmTable(w, cols, rows)
	}
	return writeTabSeparatedTable(w, cols, rows)
}

func columnHeaders(cols []Column) []string {
	headers := make([]string, len(cols))
	for i, c := range cols {
		h := c.Header
		if h == "" {
			h = c.Name
		}
		headers[i] = h
	}
	return headers
}

func writeCharmTable(w io.Writer, cols []Column, rows [][]string) error {
	headers := columnHeaders(cols)
	width := TerminalWidth(w)
	rendered := HumanTable(headers, rows, width)
	_, err := fmt.Fprintln(w, rendered)
	return err
}

func writeTabSeparatedTable(w io.Writer, cols []Column, rows [][]string) error {
	headers := columnHeaders(cols)
	if _, err := fmt.Fprintln(w, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(w, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return nil
}

package render

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/term"
)

const (
	enterAltScreen = "\x1b[?1049h\x1b[H\x1b[2J"
	exitAltScreen  = "\x1b[?1049l"
	clearScreen    = "\x1b[H\x1b[2J"
)

func HumanTable(headers []string, rows [][]string, width int) string {
	// Per CEO directive msg=c414c475 ("表格不需要边框，去掉"): drop every
	// border edge so the rendered output is purely whitespace-separated
	// while preserving lipgloss column-width alignment that distinguishes
	// the TTY path from the non-TTY tab-separated pipe path.
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle()).
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderRow(false).
		BorderColumn(false).
		Wrap(false)
	if width > 0 {
		t = t.Width(width)
	}
	return t.Render()
}

func FitLine(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	const ellipsis = "…"
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)+ellipsis) > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}

func IsTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func SupportsInteractiveTerminal(w io.Writer) bool {
	if !IsTerminal(w) {
		return false
	}
	return strings.ToLower(os.Getenv("TERM")) != "dumb"
}

func TerminalWidth(w io.Writer) int {
	if f, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(f.Fd()); err == nil && width > 0 {
			return width
		}
	}
	width, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func EnterAltScreen() string { return enterAltScreen }

func ExitAltScreen() string { return exitAltScreen }

func ClearScreen() string { return clearScreen }

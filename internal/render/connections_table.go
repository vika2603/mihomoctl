package render

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

const (
	enterAltScreen = "\x1b[?1049h\x1b[H\x1b[2J"
	exitAltScreen  = "\x1b[?1049l"
	clearScreen    = "\x1b[H\x1b[2J"
)

func HumanTable(headers []string, rows [][]string) string {
	t := table.New().
		Headers(headers...).
		Rows(rows...).
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle()).
		Wrap(false)
	return t.Render()
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

func TerminalWidth() int {
	width, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || width <= 0 {
		return 0
	}
	return width
}

func EnterAltScreen() string { return enterAltScreen }

func ExitAltScreen() string { return exitAltScreen }

func ClearScreen() string { return clearScreen }

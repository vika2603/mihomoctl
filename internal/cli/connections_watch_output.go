package cli

import (
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
	"github.com/the-super-company/mihomoctl/internal/streaming"
)

type connectionWatchEvent struct {
	Type string                `json:"type"`
	Data connectionEventOutput `json:"data"`
}

type connectionEventOutput struct {
	EventAction string             `json:"event_action"`
	ReceivedAt  string             `json:"received_at"`
	Connections []connectionOutput `json:"connections"`
}

func writeConnectionWatchEvent(out io.Writer, cfg config, opts connectionsWatchOptions, event mihomo.WatchEvent) error {
	rows := filterWatchConnections(event.Connections, opts.filter, opts.limit)
	if cfg.jsonOut {
		if len(rows) == 0 {
			return nil
		}
		return streaming.WriteNDJSON(out, connectionWatchEvent{Type: "event", Data: connectionEventOutput{
			EventAction: "snapshot",
			ReceivedAt:  event.ReceivedAt.UTC().Format(time.RFC3339),
			Connections: rows,
		}})
	}
	if opts.tui {
		return writeConnectionWatchTUI(out, opts, event, rows)
	}
	if len(rows) == 0 {
		return streaming.WriteTextLine(out, "no matching active connections")
	}
	if err := streaming.WriteTextLine(out, "received_at\tstarted_at\tsource\tdestination\tnetwork\trule\tchains\tup/down"); err != nil {
		return err
	}
	for _, c := range rows {
		if err := streaming.WriteTextLine(out, strings.Join([]string{
			event.ReceivedAt.UTC().Format(time.RFC3339),
			c.StartedAt,
			c.Source,
			c.Destination,
			c.Network,
			c.Rule,
			strings.Join(c.Chains, " > "),
			render.FormatBytes(c.UploadBytes) + "/" + render.FormatBytes(c.DownloadBytes),
		}, "\t")); err != nil {
			return err
		}
	}
	return nil
}

func writeConnectionWatchTUI(out io.Writer, opts connectionsWatchOptions, event mihomo.WatchEvent, rows []connectionOutput) error {
	text := render.ClearScreen() + renderConnectionWatchTUI(opts, event, rows, render.TerminalWidth())
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return streaming.WriteText(out, text)
}

func renderConnectionWatchTUI(opts connectionsWatchOptions, event mihomo.WatchEvent, rows []connectionOutput, width int) string {
	filter := opts.filter
	if filter == "" {
		filter = "(no filter)"
	}
	limit := "unlimited"
	if opts.limit > 0 {
		limit = strconv.Itoa(opts.limit)
	}
	lines := []string{
		"mihomoctl connections watch",
		"received_at: " + event.ReceivedAt.UTC().Format(time.RFC3339) +
			"  matches: " + strconv.Itoa(len(rows)) +
			"  filter: " + filter +
			"  limit: " + limit,
	}
	if len(rows) == 0 {
		lines = append(lines, "no matching active connections — watcher is live")
		return strings.Join(lines, "\n")
	}
	tableRows := make([][]string, 0, len(rows))
	if width > 0 && width < 60 {
		for _, c := range rows {
			tableRows = append(tableRows, []string{
				c.ID,
				c.Source,
				c.Destination,
				render.FormatBytes(c.UploadBytes) + "/" + render.FormatBytes(c.DownloadBytes),
			})
		}
		lines = append(lines, render.HumanTable([]string{"id", "source", "destination", "up/down"}, tableRows))
		return strings.Join(lines, "\n")
	}
	for _, c := range rows {
		tableRows = append(tableRows, []string{
			c.StartedAt,
			c.Source,
			c.Destination,
			c.Network,
			c.Rule,
			strings.Join(c.Chains, " > "),
			render.FormatBytes(c.UploadBytes) + "/" + render.FormatBytes(c.DownloadBytes),
		})
	}
	lines = append(lines, render.HumanTable([]string{"started_at", "source", "destination", "net", "rule", "chains", "up/down"}, tableRows))
	return strings.Join(lines, "\n")
}

func filterWatchConnections(connections []mihomo.Connection, filter string, limit int) []connectionOutput {
	if limit <= 0 {
		limit = len(connections)
		if limit == 0 {
			limit = 1
		}
	}
	opts := connectionsListOptions{limit: limit, filter: filter}
	return buildConnectionsOutput(mihomo.ConnectionsSnapshot{Connections: connections}, opts).Connections
}

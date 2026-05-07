package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
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
	rows := filterWatchConnections(event.Connections, opts.filter)
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
	if len(rows) == 0 {
		return streaming.WriteTextLine(out, "no matching active connections")
	}
	if err := streaming.WriteTextLine(out, "received_at\tstarted_at\tsource\tdestination\tnetwork\trule\tchains\tup/down"); err != nil {
		return err
	}
	for _, c := range rows {
		if err := streaming.WriteTextLine(out, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d/%d",
			event.ReceivedAt.UTC().Format(time.RFC3339), c.StartedAt, c.Source, c.Destination, c.Network, c.Rule, strings.Join(c.Chains, " > "), c.UploadBytes, c.DownloadBytes)); err != nil {
			return err
		}
	}
	return nil
}

func filterWatchConnections(connections []mihomo.Connection, filter string) []connectionOutput {
	opts := connectionsListOptions{limit: len(connections), filter: filter}
	if opts.limit == 0 {
		opts.limit = 1
	}
	return buildConnectionsOutput(mihomo.ConnectionsSnapshot{Connections: connections}, opts).Connections
}

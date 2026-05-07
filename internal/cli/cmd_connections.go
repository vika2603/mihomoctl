package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type connectionsListOptions struct {
	limit  int
	filter string
}

type connectionsOutput struct {
	Total       int                `json:"total"`
	Limit       int                `json:"limit"`
	Connections []connectionOutput `json:"connections"`
}

type connectionOutput struct {
	ID            string   `json:"id"`
	StartedAt     string   `json:"started_at"`
	Network       string   `json:"network"`
	Source        string   `json:"source"`
	Destination   string   `json:"destination"`
	Host          string   `json:"host"`
	Rule          string   `json:"rule"`
	Chains        []string `json:"chains"`
	UploadBytes   int64    `json:"upload_bytes"`
	DownloadBytes int64    `json:"download_bytes"`
	start         time.Time
}

func newConnectionsCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "connections",
		Aliases: []string{"conns"},
		Short:   "Inspect active mihomo connections",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("connections requires list")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return unknownSubcommand(cmd, args[0])
		},
	}
	cmd.AddCommand(newConnectionsListCommand(out, cfg))
	cmd.AddCommand(newConnectionsWatchCommand(out, cfg))
	return cmd
}

func newConnectionsListCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := connectionsListOptions{limit: 50}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active connection snapshots",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("connections list takes no arguments")
			}
			if opts.limit <= 0 {
				return usage("--limit must be > 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runConnectionsList(ctx, out, *cfg, client, opts)
			})
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "maximum connections to show")
	cmd.Flags().StringVar(&opts.filter, "filter", "", "substring filter against host, destination, source, or rule")
	return cmd
}

func runConnectionsList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, opts connectionsListOptions) error {
	snapshot, err := client.ListConnections(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildConnectionsOutput(snapshot, opts)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	if len(result.Connections) == 0 {
		fmt.Fprintln(out, "no active connections")
		return nil
	}
	fmt.Fprintln(out, "started_at\tsource\tdestination\tnetwork\trule\tchains\tup/down")
	for _, c := range result.Connections {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\t%s/%s\n",
			c.StartedAt, c.Source, c.Destination, c.Network, c.Rule, strings.Join(c.Chains, " > "), render.FormatBytes(c.UploadBytes), render.FormatBytes(c.DownloadBytes))
	}
	return nil
}

func buildConnectionsOutput(snapshot mihomo.ConnectionsSnapshot, opts connectionsListOptions) connectionsOutput {
	connections := make([]connectionOutput, 0, len(snapshot.Connections))
	for _, c := range snapshot.Connections {
		out := normalizeConnection(c)
		if opts.filter != "" && !connectionMatches(out, opts.filter) {
			continue
		}
		connections = append(connections, out)
	}
	sort.Slice(connections, func(i, j int) bool {
		if !connections[i].start.Equal(connections[j].start) {
			return connections[i].start.After(connections[j].start)
		}
		return connections[i].ID < connections[j].ID
	})
	total := len(connections)
	if opts.limit < len(connections) {
		connections = connections[:opts.limit]
	}
	return connectionsOutput{Total: total, Limit: opts.limit, Connections: connections}
}

func normalizeConnection(c mihomo.Connection) connectionOutput {
	startedAt := ""
	if !c.Start.IsZero() {
		startedAt = c.Start.UTC().Format(time.RFC3339)
	}
	return connectionOutput{
		ID:            c.ID,
		StartedAt:     startedAt,
		Network:       c.Metadata.Network,
		Source:        joinMaybeHostPort(c.Metadata.SourceIP, c.Metadata.SourcePort),
		Destination:   joinMaybeHostPort(c.Metadata.DestinationIP, c.Metadata.DestinationPort),
		Host:          c.Metadata.Host,
		Rule:          c.Rule,
		Chains:        c.Chains,
		UploadBytes:   c.UploadBytes,
		DownloadBytes: c.DownloadBytes,
		start:         c.Start,
	}
}

func connectionMatches(c connectionOutput, pattern string) bool {
	pattern = strings.ToLower(pattern)
	for _, value := range []string{c.Host, c.Destination, c.Source, c.Rule} {
		if strings.Contains(strings.ToLower(value), pattern) {
			return true
		}
	}
	return false
}

func joinMaybeHostPort(host, port string) string {
	if host == "" {
		return ""
	}
	if port == "" || port == "0" {
		return host
	}
	return net.JoinHostPort(host, port)
}

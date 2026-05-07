package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type groupsOutput struct {
	Total  int           `json:"total"`
	Groups []groupOutput `json:"groups"`
}

const defaultDelayURL = "http://www.gstatic.com/generate_204"

type groupDelayOptions struct {
	url          string
	delayTimeout time.Duration
}

type delayOutput struct {
	Group         string        `json:"group"`
	Type          string        `json:"type"`
	Selected      string        `json:"selected"`
	URL           string        `json:"url"`
	TestTimeoutMS int64         `json:"test_timeout_ms"`
	Results       []delayResult `json:"results"`
}

type delayResult struct {
	Node      string `json:"node"`
	LatencyMS *int   `json:"latency_ms"`
	Status    string `json:"status"`
}

func newGroupsCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "groups",
		Short: "Inspect proxy groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("groups requires list, get, or delay")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown groups subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newGroupsListCommand(out, cfg), newGroupsGetCommand(out, cfg), newGroupsDelayCommand(out, cfg))
	return cmd
}

var groupsListColumns = render.TableSpec{
	Columns: []render.Column{
		{Name: "name"},
		{Name: "type"},
		{Name: "selected"},
		{Name: "candidates"},
	},
}

func newGroupsListCommand(out io.Writer, cfg *config) *cobra.Command {
	var columnsFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proxy groups",
		Long:  "List proxy groups.\n\nInventory: GET /group.\nThis is a read-only command.\n\nAvailable --columns: " + groupsListColumns.AvailableNames() + ".",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("groups list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupsList(ctx, out, *cfg, client, columnsFlag)
			})
		},
	}
	cmd.Flags().StringVar(&columnsFlag, "columns", "", "comma-separated columns for human output (default = all). Available: "+groupsListColumns.AvailableNames())
	return cmd
}

func newGroupsGetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show one proxy group",
		Long:  "Show one proxy group.\n\nInventory: GET /group/{name}.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("groups get requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupsGet(ctx, out, *cfg, client, args[0])
			})
		},
	}
	return cmd
}

func newGroupsDelayCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := groupDelayOptions{url: defaultDelayURL, delayTimeout: 5 * time.Second}
	cmd := &cobra.Command{
		Use:   "delay <group>",
		Short: "Test candidate node latency for a proxy group",
		Long:  "Test candidate node latency for a proxy group.\n\nInventory: GET /group/{name}/delay.\nThis is a read-only probe command with an upstream fixed-selection side effect for some non-selector selectable groups.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("groups delay requires <group>")
			}
			if opts.delayTimeout <= 0 {
				return usage("--delay-timeout must be > 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupsDelay(ctx, out, *cfg, client, args[0], opts)
			})
		},
	}
	cmd.Flags().DurationVar(&opts.delayTimeout, "delay-timeout", opts.delayTimeout, "mihomo delay probe timeout")
	cmd.Flags().StringVar(&opts.url, "url", opts.url, "delay test target URL")
	return cmd
}

func runGroupsList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, columnsFlag string) error {
	groups, err := client.ListGroups(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildGroupsOutput(groups)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	cols, err := groupsListColumns.Select(render.ParseColumns(columnsFlag))
	if err != nil {
		return usage("%s", err)
	}
	if len(result.Groups) == 0 {
		fmt.Fprintln(out, "no proxy groups")
		return nil
	}
	rows := make([][]string, len(result.Groups))
	for i, g := range result.Groups {
		values := map[string]string{
			"name":       g.Name,
			"type":       g.Type,
			"selected":   emptyDash(g.Selected),
			"candidates": fmt.Sprintf("%d", len(g.Candidates)),
		}
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = values[c.Name]
		}
		rows[i] = row
	}
	return render.WriteTable(out, cols, rows)
}

func runGroupsGet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string) error {
	group, err := client.GetGroup(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	result := buildGroupOutput(group.Name, group)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s (%s)\n", result.Name, result.Type)
	fmt.Fprintf(out, "selected: %s\n", emptyDash(result.Selected))
	if len(result.Candidates) == 0 {
		fmt.Fprintln(out, "candidates: -")
		return nil
	}
	fmt.Fprintln(out, "candidates:")
	for _, node := range result.Candidates {
		marker := " "
		if node == result.Selected {
			marker = "*"
		}
		fmt.Fprintf(out, "  %s %s\n", marker, node)
	}
	return nil
}

func runGroupsDelay(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, group string, opts groupDelayOptions) error {
	proxies, err := client.ListProxies(ctx)
	if err != nil {
		return mapErr(err)
	}
	proxy, err := validateDelayGroup(proxies, group)
	if err != nil {
		return err
	}

	requestTimeout := cfg.timeout
	if !cfg.timeoutExplicit && requestTimeout <= opts.delayTimeout {
		requestTimeout = opts.delayTimeout + time.Second
	}
	delays, err := client.GroupDelay(ctx, group, mihomo.GroupDelayOptions{
		URL:            opts.url,
		DelayTimeout:   opts.delayTimeout,
		RequestTimeout: requestTimeout,
	})
	if err != nil {
		return mapErr(err)
	}

	result := buildDelayOutput(group, proxy, opts, delays)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s (%s) selected: %s\n", result.Group, result.Type, result.Selected)
	fmt.Fprintln(out, "node\tlatency_ms\tstatus")
	for _, r := range result.Results {
		latency := "-"
		if r.LatencyMS != nil {
			latency = fmt.Sprintf("%d", *r.LatencyMS)
		}
		marker := " "
		if r.Node == result.Selected {
			marker = "*"
		}
		fmt.Fprintf(out, "%s %s\t%s\t%s\n", marker, r.Node, latency, r.Status)
	}
	return nil
}

func buildGroupsOutput(groups []mihomo.Proxy) groupsOutput {
	rows := make([]groupOutput, 0, len(groups))
	for _, group := range groups {
		if len(group.All) == 0 {
			continue
		}
		rows = append(rows, buildGroupOutput(group.Name, group))
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return groupsOutput{Total: len(rows), Groups: rows}
}

func buildGroupOutput(name string, proxy mihomo.Proxy) groupOutput {
	if proxy.Name != "" {
		name = proxy.Name
	}
	return groupOutput{
		Name:       name,
		Type:       proxy.Type,
		Selected:   proxy.Now,
		Candidates: append([]string(nil), proxy.All...),
	}
}

func buildDelayOutput(group string, proxy mihomo.Proxy, opts groupDelayOptions, delays map[string]int) delayOutput {
	results := make([]delayResult, 0, len(proxy.All))
	for _, node := range proxy.All {
		if latency, ok := delays[node]; ok {
			latency := latency
			results = append(results, delayResult{Node: node, LatencyMS: &latency, Status: "ok"})
			continue
		}
		results = append(results, delayResult{Node: node, Status: "timeout"})
	}
	sort.Slice(results, func(i, j int) bool {
		left, right := resultSortLatency(results[i]), resultSortLatency(results[j])
		if left != right {
			return left < right
		}
		return results[i].Node < results[j].Node
	})
	return delayOutput{
		Group:         group,
		Type:          proxy.Type,
		Selected:      proxy.Now,
		URL:           opts.url,
		TestTimeoutMS: opts.delayTimeout.Milliseconds(),
		Results:       results,
	}
}

func resultSortLatency(r delayResult) int {
	if r.LatencyMS == nil {
		return math.MaxInt
	}
	return *r.LatencyMS
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

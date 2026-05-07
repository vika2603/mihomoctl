package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

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

func newGroupCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Run group-level operations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("group requires delay")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown group subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newGroupDelayCommand(out, cfg))
	return cmd
}

func newGroupDelayCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := groupDelayOptions{url: defaultDelayURL, delayTimeout: 5 * time.Second}
	cmd := &cobra.Command{
		Use:   "delay <group>",
		Short: "Test candidate node latency for a proxy group",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("group delay requires <group>")
			}
			if opts.delayTimeout <= 0 {
				return usage("--delay-timeout must be > 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runGroupDelay(ctx, out, *cfg, client, args[0], opts)
			})
		},
	}
	cmd.Flags().DurationVar(&opts.delayTimeout, "delay-timeout", opts.delayTimeout, "mihomo delay probe timeout")
	cmd.Flags().StringVar(&opts.url, "url", opts.url, "delay test target URL")
	return cmd
}

func runGroupDelay(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, group string, opts groupDelayOptions) error {
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

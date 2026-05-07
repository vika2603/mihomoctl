package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type rulesListOptions struct {
	limit  int
	filter string
}

type rulesOutput struct {
	Total int          `json:"total"`
	Limit int          `json:"limit"`
	Rules []ruleOutput `json:"rules"`
}

type ruleOutput struct {
	Idx     int    `json:"idx"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

func newRulesCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Inspect rule snapshots",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("rules requires list")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return unknownSubcommand(cmd, args[0])
		},
	}
	cmd.AddCommand(newRulesListCommand(out, cfg))
	return cmd
}

func newRulesListCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := rulesListOptions{limit: 50}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rule snapshots",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("rules list takes no arguments")
			}
			if opts.limit <= 0 {
				return usage("--limit must be > 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runRulesList(ctx, out, *cfg, client, opts)
			})
		},
	}
	cmd.Flags().IntVar(&opts.limit, "limit", opts.limit, "maximum rules to show")
	cmd.Flags().StringVar(&opts.filter, "filter", "", "substring filter against type, payload, or proxy")
	return cmd
}

func runRulesList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, opts rulesListOptions) error {
	rules, err := client.ListRules(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildRulesOutput(rules, opts)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	if len(result.Rules) == 0 {
		fmt.Fprintln(out, "no rules")
		return nil
	}
	fmt.Fprintln(out, "idx\ttype\tpayload\tproxy")
	for _, r := range result.Rules {
		fmt.Fprintf(out, "%d\t%s\t%s\t%s\n", r.Idx, r.Type, r.Payload, r.Proxy)
	}
	return nil
}

func buildRulesOutput(rules []mihomo.Rule, opts rulesListOptions) rulesOutput {
	out := make([]ruleOutput, 0, len(rules))
	for _, r := range rules {
		rule := ruleOutput{Idx: r.Index, Type: r.Type, Payload: r.Payload, Proxy: r.Proxy}
		if opts.filter != "" && !ruleMatches(rule, opts.filter) {
			continue
		}
		out = append(out, rule)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Idx < out[j].Idx
	})
	total := len(out)
	if opts.limit < len(out) {
		out = out[:opts.limit]
	}
	return rulesOutput{Total: total, Limit: opts.limit, Rules: out}
}

func ruleMatches(r ruleOutput, pattern string) bool {
	pattern = strings.ToLower(pattern)
	for _, value := range []string{r.Type, r.Payload, r.Proxy} {
		if strings.Contains(strings.ToLower(value), pattern) {
			return true
		}
	}
	return false
}

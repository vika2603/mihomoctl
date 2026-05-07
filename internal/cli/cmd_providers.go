package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type providersOutput struct {
	Total     int              `json:"total"`
	Limit     int              `json:"limit"`
	Providers []providerOutput `json:"providers"`
}

type providerOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	VehicleType string `json:"vehicle_type"`
	Health      string `json:"health"`
	NodeCount   int    `json:"node_count"`
	UpdatedAt   string `json:"updated_at"`
}

type providerHealthcheckOutput struct {
	Provider    string `json:"provider"`
	Type        string `json:"type"`
	VehicleType string `json:"vehicle_type"`
	Health      string `json:"health"`
	NodeCount   int    `json:"node_count"`
	UpdatedAt   string `json:"updated_at"`
	TriggeredAt string `json:"triggered_at"`
}

type providerUpdateOutput struct {
	Provider    string    `json:"provider"`
	Type        string    `json:"type"`
	VehicleType string    `json:"vehicle_type"`
	Health      string    `json:"health"`
	NodeCount   int       `json:"node_count"`
	UpdatedAt   string    `json:"updated_at"`
	Updated     bool      `json:"updated"`
	DryRun      bool      `json:"dry_run,omitempty"`
	DryRunMode  string    `json:"dry_run_mode,omitempty"`
	Risk        *riskInfo `json:"risk"`
}

type ruleProvidersOutput struct {
	Total     int                  `json:"total"`
	Providers []ruleProviderOutput `json:"providers"`
}

type ruleProviderOutput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	VehicleType string `json:"vehicle_type"`
	Behavior    string `json:"behavior"`
	RuleCount   int    `json:"rule_count"`
	UpdatedAt   string `json:"updated_at"`
}

type providerActionOptions struct {
	yes    bool
	dryRun bool
}

var proxyProvidersListColumns = render.TableSpec{
	Columns: []render.Column{
		{Name: "name"},
		{Name: "type"},
		{Name: "vehicle_type"},
		{Name: "health"},
		{Name: "node_count"},
		{Name: "updated_at"},
	},
}

var ruleProvidersListColumns = render.TableSpec{
	Columns: []render.Column{
		{Name: "name"},
		{Name: "type"},
		{Name: "vehicle_type"},
		{Name: "behavior"},
		{Name: "rule_count"},
		{Name: "updated_at"},
	},
}

func newProxyProvidersCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy-providers",
		Short: "Inspect proxy providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("proxy-providers requires list, get, update, or healthcheck")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown proxy-providers subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newProxyProvidersListCommand(out, cfg), newProxyProvidersGetCommand(out, cfg), newProxyProvidersUpdateCommand(out, cfg), newProxyProvidersHealthcheckCommand(out, cfg))
	return cmd
}

func newRuleProvidersCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rule-providers",
		Short: "Inspect rule providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("rule-providers requires list")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown rule-providers subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newRuleProvidersListCommand(out, cfg))
	return cmd
}

func newProxyProvidersListCommand(out io.Writer, cfg *config) *cobra.Command {
	var columnsFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proxy provider snapshots",
		Long:  "List proxy provider snapshots.\n\nInventory: GET /providers/proxies.\nThis is a read-only command.\n\nAvailable --columns: " + proxyProvidersListColumns.AvailableNames() + ".",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("proxy-providers list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProvidersList(ctx, out, *cfg, client, columnsFlag)
			})
		},
	}
	cmd.Flags().StringVar(&columnsFlag, "columns", "", "comma-separated columns for human output (default = all). Available: "+proxyProvidersListColumns.AvailableNames())
	return cmd
}

func newProxyProvidersGetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <name>",
		Short: "Show one proxy provider",
		Long:  "Show one proxy provider.\n\nInventory: GET /providers/proxies/{provider}.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("proxy-providers get requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxyProvidersGet(ctx, out, *cfg, client, args[0])
			})
		},
	}
	return cmd
}

func newProxyProvidersUpdateCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := providerActionOptions{}
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update one proxy provider",
		Long:  "Update one proxy provider.\n\nInventory: PUT /providers/proxies/{provider}.\nThis is a medium-impact mutation per ADR-0014 §4.2: it refetches provider contents and can change available nodes.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("proxy-providers update requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxyProvidersUpdate(ctx, out, *cfg, client, args[0], opts, time.Now)
			})
		},
	}
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "run without interactive confirmation")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "preview the provider update without contacting the mutation endpoint")
	return cmd
}

func newProxyProvidersHealthcheckCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := providerActionOptions{}
	cmd := &cobra.Command{
		Use:   "healthcheck <name>",
		Short: "Trigger a proxy provider healthcheck and show the updated summary",
		Long:  "Trigger a proxy provider healthcheck and show the updated summary.\n\nInventory: GET /providers/proxies/{provider}/healthcheck.\nThis is a low-impact side-effecting probe: it runs without a confirmation prompt, accepts --yes as a no-op, and rejects --dry-run.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("proxy-providers healthcheck requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.dryRun {
				return dryRunUnsupportedError("proxy-providers healthcheck "+args[0], "run the healthcheck")
			}
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxyProvidersHealthcheck(ctx, out, *cfg, client, args[0], time.Now)
			})
		},
	}
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "no-op; accepted for flag uniformity with higher-tier mutations")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "not supported; healthcheck has no real preview")
	return cmd
}

func newRuleProvidersListCommand(out io.Writer, cfg *config) *cobra.Command {
	var columnsFlag string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rule provider snapshots",
		Long:  "List rule provider snapshots.\n\nInventory: GET /providers/rules.\nThis is a read-only command.\n\nAvailable --columns: " + ruleProvidersListColumns.AvailableNames() + ".",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("rule-providers list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runRuleProvidersList(ctx, out, *cfg, client, columnsFlag)
			})
		},
	}
	cmd.Flags().StringVar(&columnsFlag, "columns", "", "comma-separated columns for human output (default = all). Available: "+ruleProvidersListColumns.AvailableNames())
	return cmd
}

func runProvidersList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, columnsFlag string) error {
	providers, err := client.ListProxyProviders(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildProvidersOutput(providers)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	cols, err := proxyProvidersListColumns.Select(render.ParseColumns(columnsFlag))
	if err != nil {
		return usage("%s", err)
	}
	if len(result.Providers) == 0 {
		fmt.Fprintln(out, "no proxy providers")
		return nil
	}
	rows := make([][]string, len(result.Providers))
	for i, p := range result.Providers {
		values := map[string]string{
			"name":         p.Name,
			"type":         p.Type,
			"vehicle_type": p.VehicleType,
			"health":       p.Health,
			"node_count":   fmt.Sprintf("%d", p.NodeCount),
			"updated_at":   emptyDash(p.UpdatedAt),
		}
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = values[c.Name]
		}
		rows[i] = row
	}
	return render.WriteTable(out, cols, rows)
}

func runProxyProvidersGet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string) error {
	provider, err := client.GetProxyProvider(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	result := buildProviderOutput(name, provider)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s (%s)\n", result.Name, result.VehicleType)
	fmt.Fprintf(out, "health: %s\n", result.Health)
	fmt.Fprintf(out, "nodes: %d\n", result.NodeCount)
	fmt.Fprintf(out, "updated_at: %s\n", emptyDash(result.UpdatedAt))
	return nil
}

func runProxyProvidersUpdate(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string, opts providerActionOptions, now func() time.Time) error {
	provider, err := client.GetProxyProvider(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	if opts.dryRun {
		result := buildProviderUpdateOutput(name, provider, false)
		result.DryRun = true
		result.DryRunMode = "client_simulated"
		if cfg.jsonOut {
			return render.WriteJSON(out, result)
		}
		fmt.Fprintf(out, "%s\twould update\t%s\n", result.Provider, result.Risk.Level)
		return nil
	}
	if err := confirmMediumImpact(stdinConfirmInput(), mediumConfirmOptions{
		target:  "proxy-providers update " + name,
		summary: fmt.Sprintf("Refetch proxy provider %q and replace its provider snapshot.", name),
		yes:     opts.yes,
	}); err != nil {
		return err
	}
	if err := client.UpdateProxyProvider(ctx, name); err != nil {
		return mapErr(err)
	}
	triggeredAt := now().UTC().Format(time.RFC3339)
	provider, err = client.GetProxyProvider(ctx, name)
	if err != nil {
		return mapErr(err)
	}
	result := buildProviderUpdateOutput(name, provider, true)
	result.UpdatedAt = nonEmpty(result.UpdatedAt, triggeredAt)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s\tupdated\t%s\t%d\t%s\n", result.Provider, result.Health, result.NodeCount, result.UpdatedAt)
	return nil
}

func runRuleProvidersList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, columnsFlag string) error {
	providers, err := client.ListRuleProviders(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildRuleProvidersOutput(providers)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	cols, err := ruleProvidersListColumns.Select(render.ParseColumns(columnsFlag))
	if err != nil {
		return usage("%s", err)
	}
	if len(result.Providers) == 0 {
		fmt.Fprintln(out, "no rule providers")
		return nil
	}
	rows := make([][]string, len(result.Providers))
	for i, p := range result.Providers {
		values := map[string]string{
			"name":         p.Name,
			"type":         p.Type,
			"vehicle_type": p.VehicleType,
			"behavior":     p.Behavior,
			"rule_count":   fmt.Sprintf("%d", p.RuleCount),
			"updated_at":   emptyDash(p.UpdatedAt),
		}
		row := make([]string, len(cols))
		for j, c := range cols {
			row[j] = values[c.Name]
		}
		rows[i] = row
	}
	return render.WriteTable(out, cols, rows)
}

func runProxyProvidersHealthcheck(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string, now func() time.Time) error {
	return runProvidersHealthcheck(ctx, out, cfg, client, name, now)
}

func runProvidersHealthcheck(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, name string, now func() time.Time) error {
	providers, err := client.ListProxyProviders(ctx)
	if err != nil {
		return mapErr(err)
	}
	if _, err := validateProxyProvider(providers, name); err != nil {
		return err
	}
	if err := client.HealthcheckProxyProvider(ctx, name); err != nil {
		return mapErr(err)
	}
	triggeredAt := now().UTC().Format(time.RFC3339)
	providers, err = client.ListProxyProviders(ctx)
	if err != nil {
		return mapErr(err)
	}
	provider, err := validateProxyProvider(providers, name)
	if err != nil {
		return err
	}
	result := buildProviderHealthcheckOutput(name, provider, triggeredAt)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
		result.Provider, result.Type, result.VehicleType, result.Health, result.NodeCount, result.UpdatedAt, result.TriggeredAt)
	return nil
}

func buildProvidersOutput(providers map[string]mihomo.ProxyProvider) providersOutput {
	rows := make([]providerOutput, 0, len(providers))
	for name, p := range providers {
		rows = append(rows, buildProviderOutput(name, p))
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return providersOutput{Total: len(rows), Limit: len(rows), Providers: rows}
}

func buildRuleProvidersOutput(providers map[string]mihomo.RuleProvider) ruleProvidersOutput {
	rows := make([]ruleProviderOutput, 0, len(providers))
	for name, p := range providers {
		rows = append(rows, buildRuleProviderOutput(name, p))
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Name < rows[j].Name
	})
	return ruleProvidersOutput{Total: len(rows), Providers: rows}
}

func buildProviderUpdateOutput(name string, p mihomo.ProxyProvider, updated bool) providerUpdateOutput {
	row := buildProviderOutput(name, p)
	return providerUpdateOutput{
		Provider:    row.Name,
		Type:        row.Type,
		VehicleType: row.VehicleType,
		Health:      row.Health,
		NodeCount:   row.NodeCount,
		UpdatedAt:   row.UpdatedAt,
		Updated:     updated,
		Risk:        mediumRisk("Refetches one proxy provider and may replace its available node snapshot."),
	}
}

func buildProviderOutput(name string, p mihomo.ProxyProvider) providerOutput {
	if p.Name != "" {
		name = p.Name
	}
	return providerOutput{
		Name:        name,
		Type:        "Proxy",
		VehicleType: p.VehicleType,
		Health:      providerHealth(p),
		NodeCount:   len(p.Proxies),
		UpdatedAt:   formatTime(p.UpdatedAt),
	}
}

func buildRuleProviderOutput(name string, p mihomo.RuleProvider) ruleProviderOutput {
	if p.Name != "" {
		name = p.Name
	}
	return ruleProviderOutput{
		Name:        name,
		Type:        "Rule",
		VehicleType: p.VehicleType,
		Behavior:    p.Behavior,
		RuleCount:   p.RuleCount,
		UpdatedAt:   formatTime(p.UpdatedAt),
	}
}

func nonEmpty(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func buildProviderHealthcheckOutput(name string, p mihomo.ProxyProvider, triggeredAt string) providerHealthcheckOutput {
	row := buildProviderOutput(name, p)
	return providerHealthcheckOutput{
		Provider:    row.Name,
		Type:        row.Type,
		VehicleType: row.VehicleType,
		Health:      row.Health,
		NodeCount:   row.NodeCount,
		UpdatedAt:   row.UpdatedAt,
		TriggeredAt: triggeredAt,
	}
}

func validateProxyProvider(providers map[string]mihomo.ProxyProvider, name string) (mihomo.ProxyProvider, error) {
	if p, ok := providers[name]; ok {
		return p, nil
	}
	available := make([]string, 0, len(providers))
	for name := range providers {
		available = append(available, name)
	}
	sort.Strings(available)
	return mihomo.ProxyProvider{}, &cliError{Code: exitNotFound, Message: fmt.Sprintf("proxy provider %q not found, available: %s", name, strings.Join(available, ", "))}
}

func providerHealth(p mihomo.ProxyProvider) string {
	if len(p.Proxies) == 0 {
		return "unknown"
	}
	for _, proxy := range p.Proxies {
		if proxy.Alive {
			return "healthy"
		}
	}
	return "unhealthy"
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

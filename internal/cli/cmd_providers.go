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

func newProvidersCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "providers",
		Short: "Inspect proxy providers",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("providers requires list or healthcheck")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown providers subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newProvidersListCommand(out, cfg), newProvidersHealthcheckCommand(out, cfg))
	return cmd
}

func newProvidersListCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proxy provider snapshots",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("providers list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProvidersList(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func newProvidersHealthcheckCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "healthcheck <name>",
		Short: "Trigger a proxy provider healthcheck and show the updated summary",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("providers healthcheck requires <name>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProvidersHealthcheck(ctx, out, *cfg, client, args[0], time.Now)
			})
		},
	}
	return cmd
}

func runProvidersList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	providers, err := client.ListProxyProviders(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := buildProvidersOutput(providers)
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	if len(result.Providers) == 0 {
		fmt.Fprintln(out, "no proxy providers")
		return nil
	}
	fmt.Fprintln(out, "name\ttype\tvehicle_type\thealth\tnode_count\tupdated_at")
	for _, p := range result.Providers {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%d\t%s\n", p.Name, p.Type, p.VehicleType, p.Health, p.NodeCount, p.UpdatedAt)
	}
	return nil
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

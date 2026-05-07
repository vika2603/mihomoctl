package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

func newStatusCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show mode, mihomo version, and current selection for each selectable group",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			if len(args) != 0 {
				return usage("status takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return cmdStatus(ctx, out, *cfg, client, args)
			})
		},
	}
	return cmd
}

func newProxyCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy",
		Short: "List proxy groups, select a node, or probe node latency",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("proxy requires list, set, or delay")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown proxy subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newProxyListCommand(out, cfg), newProxySetCommand(out, cfg), newProxyDelayCommand(out, cfg))
	return cmd
}

func newProxyListCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List selectable proxy groups and nodes",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("proxy list takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxyList(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func newProxySetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <group> <node>",
		Short: "Select a node for a selector group",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return usage("proxy set requires <group> <node>")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxySet(ctx, out, *cfg, client, args[0], args[1])
			})
		},
	}
	return cmd
}

func newModeCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mode",
		Short: "Show or change current mihomo mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("mode requires get or set")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown mode subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newModeGetCommand(out, cfg), newModeSetCommand(out, cfg))
	return cmd
}

func newModeGetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Show current mode",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("mode get takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runModeGet(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func newModeSetCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <rule|global|direct>",
		Short: "Change current mode",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("mode set requires <rule|global|direct>")
			}
			if !oneOfMode(args[0]) {
				return usage("invalid mode %q; expected rule, global, or direct", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runModeSet(ctx, out, *cfg, client, args[0])
			})
		},
	}
	return cmd
}

func cmdStatus(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, args []string) error {
	if len(args) != 0 {
		return usage("status takes no arguments")
	}
	conf, err := client.GetConfigs(ctx)
	if err != nil {
		return mapErr(err)
	}
	version, err := client.Version(ctx)
	if err != nil {
		return mapErr(err)
	}
	proxies, err := client.ListProxies(ctx)
	if err != nil {
		return mapErr(err)
	}

	groups := selectableGroups(proxies)
	if cfg.jsonOut {
		return render.WriteJSON(out, map[string]any{
			"mode":    conf.Mode,
			"version": version.Version,
			"groups":  groups,
		})
	}

	fmt.Fprintf(out, "mode: %s\n", conf.Mode)
	fmt.Fprintf(out, "version: %s\n", version.Version)
	fmt.Fprintln(out, "groups:")
	for _, g := range groups {
		fmt.Fprintf(out, "  %s: %s\n", g.Name, g.Selected)
	}
	return nil
}

func runModeGet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	conf, err := client.GetConfigs(ctx)
	if err != nil {
		return mapErr(err)
	}
	if cfg.jsonOut {
		return render.WriteJSON(out, map[string]string{"mode": conf.Mode})
	}
	fmt.Fprintln(out, conf.Mode)
	return nil
}

func runModeSet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, mode string) error {
	conf, err := client.GetConfigs(ctx)
	if err != nil {
		return mapErr(err)
	}
	if err := client.PatchMode(ctx, mode); err != nil {
		return mapErr(err)
	}
	if cfg.jsonOut {
		return render.WriteJSON(out, map[string]string{"mode": mode, "previous": conf.Mode})
	}
	fmt.Fprintf(out, "mode: %s", mode)
	if conf.Mode != "" {
		fmt.Fprintf(out, " (was %s)", conf.Mode)
	}
	fmt.Fprintln(out)
	return nil
}

func runProxyList(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	proxies, err := client.ListProxies(ctx)
	if err != nil {
		return mapErr(err)
	}
	groups := selectableGroups(proxies)
	if cfg.jsonOut {
		return render.WriteJSON(out, map[string]any{"groups": groups})
	}
	for _, g := range groups {
		fmt.Fprintf(out, "%s", g.Name)
		if g.Selected != "" {
			fmt.Fprintf(out, " -> %s", g.Selected)
		}
		fmt.Fprintln(out)
		for _, node := range g.Candidates {
			marker := " "
			if node == g.Selected {
				marker = "*"
			}
			fmt.Fprintf(out, "  %s %s\n", marker, node)
		}
	}
	return nil
}

func runProxySet(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, group, node string) error {
	proxies, err := client.ListProxies(ctx)
	if err != nil {
		return mapErr(err)
	}
	previous, err := validateSelection(proxies, group, node)
	if err != nil {
		return err
	}
	if err := client.SelectProxy(ctx, group, node); err != nil {
		return mapErr(err)
	}
	if cfg.jsonOut {
		return render.WriteJSON(out, map[string]string{"group": group, "selected": node, "previous": previous})
	}
	fmt.Fprintf(out, "%s: %s", group, node)
	if previous != "" {
		fmt.Fprintf(out, " (was %s)", previous)
	}
	fmt.Fprintln(out)
	return nil
}

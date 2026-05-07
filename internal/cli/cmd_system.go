package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type systemPingOutput struct {
	OK         bool   `json:"ok"`
	Controller string `json:"controller"`
	Version    string `json:"version"`
}

type systemVersionOutput struct {
	Version string `json:"version"`
}

func newSystemCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Inspect mihomo controller runtime information",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("system requires ping or version")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown system subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newSystemPingCommand(out, cfg), newSystemVersionCommand(out, cfg))
	return cmd
}

func newSystemPingCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping",
		Short: "Check that the mihomo controller is reachable",
		Long:  "Check that the mihomo controller is reachable.\n\nInventory: GET /version.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("system ping takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runSystemPing(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func newSystemVersionCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show mihomo controller version",
		Long:  "Show mihomo controller version.\n\nInventory: GET /version.\nThis is a read-only command.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("system version takes no arguments")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runSystemVersion(ctx, out, *cfg, client)
			})
		},
	}
	return cmd
}

func runSystemPing(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	version, err := client.Version(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := systemPingOutput{OK: true, Controller: client.Endpoint(), Version: version.Version}
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintln(out, "ok: true")
	fmt.Fprintf(out, "controller: %s\n", result.Controller)
	fmt.Fprintf(out, "version: %s\n", result.Version)
	return nil
}

func runSystemVersion(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client) error {
	version, err := client.Version(ctx)
	if err != nil {
		return mapErr(err)
	}
	result := systemVersionOutput{Version: version.Version}
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintln(out, result.Version)
	return nil
}

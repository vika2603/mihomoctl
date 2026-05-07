package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

const (
	defaultEndpoint = "http://127.0.0.1:9090"
	defaultTimeout  = 5 * time.Second
)

const (
	exitOK       = render.ExitOK
	exitUsage    = render.ExitUsage
	exitNotFound = render.ExitNotFound
	exitSoftware = render.ExitSoftware
	exitSystem   = render.ExitSystem
	exitCantOut  = render.ExitCantOut
	exitTempFail = render.ExitTempFail
	exitNoPerm   = render.ExitNoPerm
)

type config struct {
	endpoint        string
	secret          string
	jsonOut         bool
	timeout         time.Duration
	timeoutExplicit bool
}

func Run(args []string, out, errOut io.Writer) int {
	if err, cfg := runWithConfig(args, out); err != nil {
		return render.RenderError(err, cfg.jsonOut, errOut)
	}
	return exitOK
}

func run(args []string, out io.Writer) error {
	err, _ := runWithConfig(args, out)
	return err
}

func runWithConfig(args []string, out io.Writer) (error, config) {
	cmd, cfg := newRootCommandWithConfig(out)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cmd.SetContext(ctx)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		return normalizeCobraErr(err), *cfg
	}
	return nil, *cfg
}

func newRootCommand(out io.Writer) *cobra.Command {
	cmd, _ := newRootCommandWithConfig(out)
	return cmd
}

func newRootCommandWithConfig(out io.Writer) (*cobra.Command, *config) {
	cfg := &config{
		endpoint: getenvDefault("MIHOMOCTL_ENDPOINT", defaultEndpoint),
		timeout:  defaultTimeout,
	}
	root := &cobra.Command{
		Use:                "mihomoctl [--endpoint URL] [--secret VALUE|-s VALUE] [--json] [--timeout DURATION] <command>",
		Short:              "Control a local mihomo external-controller",
		SilenceUsage:       true,
		SilenceErrors:      true,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
		DisableSuggestions: true,
	}
	root.SetOut(out)
	root.SetErr(io.Discard)
	root.PersistentFlags().StringVar(&cfg.endpoint, "endpoint", cfg.endpoint, "mihomo external-controller endpoint")
	root.PersistentFlags().StringVarP(&cfg.secret, "secret", "s", "", "mihomo secret; prefer MIHOMOCTL_SECRET to avoid shell history/process-list leaks")
	root.PersistentFlags().BoolVar(&cfg.jsonOut, "json", false, "emit JSON output")
	root.PersistentFlags().DurationVar(&cfg.timeout, "timeout", cfg.timeout, "request timeout")
	root.AddCommand(newStatusCommand(out, cfg), newSystemCommand(out, cfg), newProxyCommand(out, cfg), newModeCommand(out, cfg), newGroupsCommand(out, cfg), newProxyProvidersCommand(out, cfg), newRuleProvidersCommand(out, cfg), newConnectionsCommand(out, cfg), newRulesCommand(out, cfg), newDNSCommand(out, cfg), newCacheCommand(out, cfg), newManCommand())
	return root, cfg
}

func newClient(cfg config) (*mihomo.Client, error) {
	client, err := mihomo.New(cfg.endpoint, cfg.secret, cfg.timeout)
	if err != nil {
		return nil, usage("%v", err)
	}
	return client, nil
}

func normalizeCobraErr(err error) error {
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	return usage("%v", err)
}

func getenvDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func runWithClient(cmd *cobra.Command, cfg *config, fn func(context.Context, *mihomo.Client) error) error {
	resolved := *cfg
	flags := cmd.Root().PersistentFlags()
	resolved.timeoutExplicit = flags.Changed("timeout")
	if !flags.Changed("secret") {
		resolved.secret = os.Getenv("MIHOMOCTL_SECRET")
	}
	client, err := newClient(resolved)
	if err != nil {
		return err
	}
	return fn(cmd.Context(), client)
}

func commandHelp(cmd *cobra.Command, args []string) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		return cmd.Help()
	}
	return nil
}

func oneOfMode(mode string) bool {
	return mode == "rule" || mode == "global" || mode == "direct"
}

func hasHelpArg(args []string) bool {
	for _, arg := range args {
		if arg == "help" || arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func unknownCommandError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown command")
}

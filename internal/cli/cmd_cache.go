package cli

import (
	"context"
	"io"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

// cacheClearLeafOptions captures per-leaf safety flags shared by every
// `cache clear <target>` subcommand. ADR-0014 §4.1 ranks cache clear as a
// low-impact mutation: no confirmation prompt, --yes is an optional no-op for
// flag uniformity in automation, and --dry-run is rejected with an actionable
// usage error so callers do not silently believe the cache was previewed.
type cacheClearLeafOptions struct {
	yes    bool
	dryRun bool
}

func newCacheCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Clear low-impact mihomo runtime caches",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("cache requires clear")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown cache subcommand %q", args[0])
		},
	}
	cmd.AddCommand(newCacheClearCommand(out, cfg))
	return cmd
}

func newCacheClearCommand(out io.Writer, cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Flush fakeip, DNS, or all mihomo caches",
		Long: `Flush low-impact mihomo runtime caches.

cache clear is classified low-impact in the v1.0 mutation safety matrix
(ADR-0014 §4.1): the runtime keeps configuration and active connections
untouched, so the command runs immediately without a confirmation prompt.

Safety contract:
  --yes      accepted for flag uniformity with higher-tier mutations; no-op.
  --dry-run  not supported; cache clear has no real preview. Drop the flag
             to perform the flush.

Valid targets:
  fakeip  flush the fake-IP cache
  dns     flush the DNS resolver cache
  all     flush fakeip first, then DNS

Examples:
  mihomoctl cache clear dns
  mihomoctl cache clear all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("cache clear requires a target. Use 'cache clear fakeip', 'cache clear dns', or 'cache clear all'.")
			}
			if err := commandHelp(cmd, args); err != nil || hasHelpArg(args) {
				return err
			}
			return usage("unknown cache clear target %q", args[0])
		},
	}
	cmd.AddCommand(newCacheClearLeafCommand(out, cfg, "fakeip", "Flush mihomo fakeip cache"))
	cmd.AddCommand(newCacheClearLeafCommand(out, cfg, "dns", "Flush mihomo DNS resolver cache"))
	cmd.AddCommand(newCacheClearLeafCommand(out, cfg, "all", "Flush fakeip and DNS caches"))
	return cmd
}

func newCacheClearLeafCommand(out io.Writer, cfg *config, target, short string) *cobra.Command {
	opts := cacheClearLeafOptions{}
	cmd := &cobra.Command{
		Use:   target,
		Short: short,
		Long:  cacheClearLong(target),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("cache clear %s takes no arguments", target)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.dryRun {
				return dryRunUnsupportedError("cache clear " + target)
			}
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runCacheClear(ctx, out, *cfg, client, target)
			})
		},
	}
	cmd.Flags().BoolVar(&opts.yes, "yes", false, "no-op; accepted for flag uniformity with higher-tier mutations")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "not supported; cache clear has no real preview")
	return cmd
}

func cacheClearLong(target string) string {
	const safetyTrailer = "\n\nSafety: low-impact mutation (ADR-0014 §4.1). Runs immediately, no confirmation prompt. --yes is an accepted no-op; --dry-run is rejected with an actionable usage error."
	switch target {
	case "fakeip":
		return "Flush mihomo's fakeip cache. Active connections and configuration are not changed." + safetyTrailer
	case "dns":
		return "Flush mihomo's DNS resolver cache. Future DNS lookups may be repeated; active connections are not changed." + safetyTrailer
	case "all":
		return "Flush fakeip first, then DNS. If one cache fails, mihomoctl reports a partial failure with per-cache results." + safetyTrailer
	default:
		return ""
	}
}

func runCacheClear(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, target string) error {
	var result cacheOutput
	var err error
	switch target {
	case "fakeip":
		err = client.FlushFakeIPCache(ctx)
		result = cacheOutput{Cache: "fakeip", Cleared: err == nil}
	case "dns":
		err = client.FlushDNSCache(ctx)
		result = cacheOutput{Cache: "dns", Cleared: err == nil}
	case "all":
		result, err = clearAllCaches(ctx, client)
	default:
		return usage("unknown cache clear target %q", target)
	}
	if err != nil {
		if target == "all" {
			if !cfg.jsonOut {
				writeCacheHuman(out, result)
			}
			return err
		}
		return mapErr(err)
	}
	result.Risk = lowRisk(cacheClearRiskSummary(target))
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	writeCacheHuman(out, result)
	return nil
}

func cacheClearRiskSummary(target string) string {
	if target == "all" {
		return "Flushes fakeip then DNS runtime caches only; configuration and active connections are not changed."
	}
	return "Flushes the runtime cache only; configuration and active connections are not changed."
}

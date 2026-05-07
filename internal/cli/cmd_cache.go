package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

type cacheOutput struct {
	Cache   string        `json:"cache"`
	Cleared bool          `json:"cleared"`
	Results []cacheResult `json:"results,omitempty"`
}

type cacheResult struct {
	Cache   string     `json:"cache"`
	Cleared bool       `json:"cleared"`
	Error   *errorBody `json:"error,omitempty"`
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
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usage("cache clear requires fakeip, dns, or all")
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
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runCacheClear(ctx, out, *cfg, client, target)
			})
		},
	}
	return cmd
}

func cacheClearLong(target string) string {
	switch target {
	case "fakeip":
		return "Flush mihomo's fakeip cache. Active connections and configuration are not changed."
	case "dns":
		return "Flush mihomo's DNS resolver cache. Future DNS lookups may be repeated; active connections are not changed."
	case "all":
		return "Flush fakeip first, then DNS. If one cache fails, mihomoctl reports a partial failure with per-cache results."
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
	if cfg.jsonOut {
		return writeJSON(out, result)
	}
	writeCacheHuman(out, result)
	return nil
}

func clearAllCaches(ctx context.Context, client *mihomo.Client) (cacheOutput, error) {
	results := []cacheResult{
		clearCacheTarget(ctx, client, "fakeip"),
		clearCacheTarget(ctx, client, "dns"),
	}
	out := cacheOutput{Cache: "all", Cleared: true, Results: results}
	for _, result := range results {
		if !result.Cleared {
			out.Cleared = false
		}
	}
	if out.Cleared {
		return out, nil
	}
	return out, &cliError{
		code:    exitTempFail,
		msg:     "cache clear partially failed; retry the failed cache target",
		errCode: "cache_partial_failure",
		details: map[string]any{"cache": "all", "results": results},
	}
}

func clearCacheTarget(ctx context.Context, client *mihomo.Client, target string) cacheResult {
	var err error
	switch target {
	case "fakeip":
		err = client.FlushFakeIPCache(ctx)
	case "dns":
		err = client.FlushDNSCache(ctx)
	default:
		err = fmt.Errorf("unknown cache target %q", target)
	}
	if err == nil {
		return cacheResult{Cache: target, Cleared: true}
	}
	ce := toErrorBody(mapErr(err))
	return cacheResult{Cache: target, Cleared: false, Error: &ce}
}

func writeCacheHuman(out io.Writer, result cacheOutput) {
	if result.Cache == "all" {
		for _, item := range result.Results {
			if item.Cleared {
				fmt.Fprintf(out, "%s\tcleared\n", cacheDisplayName(item.Cache))
				continue
			}
			msg := "failed"
			if item.Error != nil && item.Error.Message != "" {
				msg = item.Error.Message
			}
			fmt.Fprintf(out, "%s\tfailed\t%s\n", cacheDisplayName(item.Cache), msg)
		}
		return
	}
	if result.Cleared {
		fmt.Fprintf(out, "flushed mihomo %s cache\n", cacheDisplayName(result.Cache))
	}
}

func cacheDisplayName(cache string) string {
	switch strings.ToLower(cache) {
	case "fakeip":
		return "fakeip"
	case "dns":
		return "dns"
	default:
		return cache
	}
}

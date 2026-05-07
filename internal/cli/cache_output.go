package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

type cacheOutput struct {
	Cache   string        `json:"cache"`
	Cleared bool          `json:"cleared"`
	Results []cacheResult `json:"results,omitempty"`
}

type cacheResult struct {
	Cache   string            `json:"cache"`
	Cleared bool              `json:"cleared"`
	Error   *render.ErrorBody `json:"error,omitempty"`
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
	return out, render.NewError(exitTempFail, "cache clear partially failed; retry the failed cache target", "cache_partial_failure", map[string]any{"cache": "all", "results": results})
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
	ce := render.ToErrorBody(mapErr(err))
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

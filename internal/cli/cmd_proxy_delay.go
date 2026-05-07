package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
)

const defaultDelayExpectedStatus = "204"

type proxyDelayOptions struct {
	url            string
	expectedStatus string
	delayTimeout   time.Duration
}

type proxyDelayOutput struct {
	Proxy          string `json:"proxy"`
	URL            string `json:"url"`
	ExpectedStatus string `json:"expected_status"`
	TestTimeoutMS  int64  `json:"test_timeout_ms"`
	DelayMS        int    `json:"delay_ms"`
}

func newProxyDelayCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := proxyDelayOptions{
		url:            defaultDelayURL,
		expectedStatus: defaultDelayExpectedStatus,
		delayTimeout:   5 * time.Second,
	}
	cmd := &cobra.Command{
		Use:   "delay <node>",
		Short: "Test latency for one proxy node",
		Long:  "Test latency for one proxy node.\n\nInventory: GET /proxies/{name}/delay.\nThis is a read-only probe command with upstream network side effects only.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return usage("proxy delay requires <node>")
			}
			if opts.delayTimeout <= 0 {
				return usage("--delay-timeout must be > 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runProxyDelay(ctx, out, *cfg, client, args[0], opts)
			})
		},
	}
	cmd.Flags().DurationVar(&opts.delayTimeout, "delay-timeout", opts.delayTimeout, "mihomo delay probe timeout")
	cmd.Flags().StringVar(&opts.url, "url", opts.url, "delay test target URL")
	cmd.Flags().StringVar(&opts.expectedStatus, "expected", opts.expectedStatus, "expected HTTP status range for the delay probe")
	return cmd
}

func runProxyDelay(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, proxy string, opts proxyDelayOptions) error {
	requestTimeout := cfg.timeout
	if !cfg.timeoutExplicit && requestTimeout <= opts.delayTimeout {
		requestTimeout = opts.delayTimeout + time.Second
	}
	delay, err := client.ProxyDelay(ctx, proxy, mihomo.ProxyDelayOptions{
		URL:            opts.url,
		ExpectedStatus: opts.expectedStatus,
		DelayTimeout:   opts.delayTimeout,
		RequestTimeout: requestTimeout,
	})
	if err != nil {
		return mapErr(err)
	}
	result := proxyDelayOutput{
		Proxy:          proxy,
		URL:            opts.url,
		ExpectedStatus: opts.expectedStatus,
		TestTimeoutMS:  opts.delayTimeout.Milliseconds(),
		DelayMS:        delay,
	}
	if cfg.jsonOut {
		return render.WriteJSON(out, result)
	}
	fmt.Fprintln(out, "proxy\tlatency_ms\turl")
	fmt.Fprintf(out, "%s\t%d\t%s\n", result.Proxy, result.DelayMS, result.URL)
	return nil
}

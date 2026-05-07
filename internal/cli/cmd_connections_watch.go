package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
	"github.com/the-super-company/mihomoctl/internal/render"
	"github.com/the-super-company/mihomoctl/internal/streaming"
)

type connectionsWatchOptions struct {
	filter               string
	interval             time.Duration
	noReconnect          bool
	maxReconnectAttempts int
}

func newConnectionsWatchCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := connectionsWatchOptions{
		interval:             time.Second,
		maxReconnectAttempts: 100,
	}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream active connection snapshots from mihomo",
		Long:  "Stream active mihomo connection snapshots over WebSocket until interrupted. Filters are applied locally after each event is received.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				return usage("connections watch takes no arguments")
			}
			if opts.interval <= 0 {
				return usage("--interval must be > 0")
			}
			if opts.maxReconnectAttempts < 0 {
				return usage("--max-reconnect-attempts must be >= 0")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWithClient(cmd, cfg, func(ctx context.Context, client *mihomo.Client) error {
				return runConnectionsWatch(ctx, out, *cfg, client, opts)
			})
		},
	}
	cmd.Flags().StringVar(&opts.filter, "filter", "", "CLI-local substring filter against host, destination, source, or rule")
	cmd.Flags().DurationVar(&opts.interval, "interval", opts.interval, "mihomo websocket poll interval")
	cmd.Flags().BoolVar(&opts.noReconnect, "no-reconnect", false, "do not reconnect after websocket disconnects")
	cmd.Flags().IntVar(&opts.maxReconnectAttempts, "max-reconnect-attempts", opts.maxReconnectAttempts, "maximum consecutive reconnect failures before exiting; 0 means unbounded")
	return cmd
}

func runConnectionsWatch(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, opts connectionsWatchOptions) error {
	failures := 0
	for {
		hadEvent, err := watchOnce(ctx, out, cfg, client, opts)
		if err == nil || errors.Is(err, context.Canceled) {
			return nil
		}
		if ctx.Err() != nil {
			return nil
		}
		if hadEvent {
			failures = 0
		}
		if opts.noReconnect {
			return mapWatchErr(err)
		}
		failures++
		if opts.maxReconnectAttempts > 0 && failures > opts.maxReconnectAttempts {
			return streamErrorAndSuppress(out, cfg, exitTempFail, "reconnect_exhausted", "mihomo websocket reconnect attempts exhausted", map[string]any{"attempts": failures - 1})
		}
		if cfg.jsonOut {
			if writeErr := streaming.WriteError(out, render.NewError(exitTempFail, mapWatchErr(err).Error(), "websocket_disconnected", nil)); writeErr != nil {
				return writeErr
			}
		}
		if err := streaming.SleepReconnect(ctx, failures); err != nil {
			return nil
		}
	}
}

func watchOnce(ctx context.Context, out io.Writer, cfg config, client *mihomo.Client, opts connectionsWatchOptions) (bool, error) {
	watchCtx := ctx
	if cfg.timeout > 0 {
		var cancel context.CancelFunc
		watchCtx, cancel = context.WithTimeout(ctx, cfg.timeout)
		defer cancel()
	}
	watch, err := client.WatchConnections(watchCtx, mihomo.ConnectionsWatchOptions{Interval: opts.interval})
	if err != nil {
		return false, err
	}
	defer watch.Close()
	return streaming.ConsumeLatest(ctx, cfg.timeout, watch.Read, func(event mihomo.WatchEvent) error {
		return writeConnectionWatchEvent(out, cfg, opts, event)
	})
}

func streamErrorAndSuppress(out io.Writer, cfg config, code int, errCode, msg string, details any) error {
	ce := render.NewError(code, msg, errCode, details)
	if cfg.jsonOut {
		if err := streaming.WriteError(out, ce); err != nil {
			return err
		}
		ce.SuppressRender = true
	}
	return ce
}

func mapWatchErr(err error) error {
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return render.NewError(exitOK, "interrupted", "", nil)
	}
	var me *mihomo.Error
	if errors.As(err, &me) {
		return mapErr(err)
	}
	return render.NewError(exitTempFail, fmt.Sprintf("mihomo websocket stream disconnected: %v", err), "websocket_disconnected", nil)
}

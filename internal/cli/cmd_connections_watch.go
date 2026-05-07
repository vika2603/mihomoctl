package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

type connectionsWatchOptions struct {
	filter               string
	interval             time.Duration
	noReconnect          bool
	maxReconnectAttempts int
}

type connectionWatchEvent struct {
	Type string                `json:"type"`
	Data connectionEventOutput `json:"data"`
}

type connectionWatchError struct {
	Type  string    `json:"type"`
	Error errorBody `json:"error"`
}

type connectionEventOutput struct {
	EventAction string             `json:"event_action"`
	ReceivedAt  string             `json:"received_at"`
	Connections []connectionOutput `json:"connections"`
}

func newConnectionsWatchCommand(out io.Writer, cfg *config) *cobra.Command {
	opts := connectionsWatchOptions{
		interval:             time.Second,
		maxReconnectAttempts: 100,
	}
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Stream active connection changes from mihomo",
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
			return writeStreamErrorAndReturn(out, cfg, exitTempFail, "reconnect_exhausted", "mihomo websocket reconnect attempts exhausted", map[string]any{"attempts": failures - 1})
		}
		if cfg.jsonOut {
			if writeErr := writeStreamError(out, &cliError{code: exitTempFail, msg: mapWatchErr(err).Error(), errCode: "websocket_disconnected"}); writeErr != nil {
				return writeErr
			}
		}
		if err := sleepReconnect(ctx, failures); err != nil {
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
	hadEvent := false
	for {
		readCtx := ctx
		if cfg.timeout > 0 {
			var cancel context.CancelFunc
			readCtx, cancel = context.WithTimeout(ctx, cfg.timeout)
			event, err := watch.Read(readCtx)
			cancel()
			if err != nil {
				return hadEvent, err
			}
			if err := writeConnectionWatchEvent(out, cfg, opts, event); err != nil {
				return hadEvent, err
			}
			hadEvent = true
			continue
		}
		event, err := watch.Read(readCtx)
		if err != nil {
			return hadEvent, err
		}
		if err := writeConnectionWatchEvent(out, cfg, opts, event); err != nil {
			return hadEvent, err
		}
		hadEvent = true
	}
}

func writeConnectionWatchEvent(out io.Writer, cfg config, opts connectionsWatchOptions, event mihomo.WatchEvent) error {
	rows := filterWatchConnections(event.Connections, opts.filter)
	if cfg.jsonOut {
		if len(rows) == 0 {
			return nil
		}
		return writeNDJSON(out, connectionWatchEvent{Type: "event", Data: connectionEventOutput{
			EventAction: "snapshot",
			ReceivedAt:  event.ReceivedAt.UTC().Format(time.RFC3339),
			Connections: rows,
		}})
	}
	if len(rows) == 0 {
		return writeTextLine(out, "no matching active connections")
	}
	if err := writeTextLine(out, "received_at\tstarted_at\tsource\tdestination\tnetwork\trule\tchains\tup/down"); err != nil {
		return err
	}
	for _, c := range rows {
		if err := writeTextLine(out, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d/%d",
			event.ReceivedAt.UTC().Format(time.RFC3339), c.StartedAt, c.Source, c.Destination, c.Network, c.Rule, strings.Join(c.Chains, " > "), c.UploadBytes, c.DownloadBytes)); err != nil {
			return err
		}
	}
	return nil
}

func filterWatchConnections(connections []mihomo.Connection, filter string) []connectionOutput {
	opts := connectionsListOptions{limit: len(connections), filter: filter}
	if opts.limit == 0 {
		opts.limit = 1
	}
	return buildConnectionsOutput(mihomo.ConnectionsSnapshot{Connections: connections}, opts).Connections
}

func writeStreamErrorAndReturn(out io.Writer, cfg config, code int, errCode, msg string, details any) error {
	ce := &cliError{code: code, msg: msg, errCode: errCode, details: details}
	if cfg.jsonOut {
		if err := writeStreamError(out, ce); err != nil {
			return err
		}
		ce.suppressRender = true
	}
	return ce
}

func mapWatchErr(err error) error {
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	if errors.Is(err, context.Canceled) {
		return &cliError{code: exitOK, msg: "interrupted"}
	}
	var me *mihomo.Error
	if errors.As(err, &me) {
		return mapErr(err)
	}
	return &cliError{code: exitTempFail, msg: fmt.Sprintf("mihomo websocket stream disconnected: %v", err), errCode: "websocket_disconnected"}
}

func writeStreamError(out io.Writer, err error) error {
	if err := writeNDJSON(out, connectionWatchError{Type: "error", Error: toErrorBody(err)}); err != nil {
		return err
	}
	return nil
}

func writeNDJSON(out io.Writer, v any) error {
	if err := json.NewEncoder(out).Encode(v); err != nil {
		if isBrokenPipe(err) {
			return &cliError{code: exitOK, msg: "broken stdout pipe"}
		}
		return &cliError{code: exitCantOut, msg: fmt.Sprintf("cannot write stream output: %v", err), errCode: "output_error"}
	}
	return nil
}

func writeTextLine(out io.Writer, line string) error {
	if _, err := fmt.Fprintln(out, line); err != nil {
		if isBrokenPipe(err) {
			return &cliError{code: exitOK, msg: "broken stdout pipe"}
		}
		return &cliError{code: exitCantOut, msg: fmt.Sprintf("cannot write stream output: %v", err), errCode: "output_error"}
	}
	return nil
}

func isBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

func sleepReconnect(ctx context.Context, failures int) error {
	timer := time.NewTimer(reconnectDelay(failures))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func reconnectDelay(failures int) time.Duration {
	steps := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second}
	if failures <= 0 {
		failures = 1
	}
	delay := steps[len(steps)-1]
	if failures <= len(steps) {
		delay = steps[failures-1]
	}
	return delay + time.Duration(rand.Int64N(int64(100*time.Millisecond)))
}

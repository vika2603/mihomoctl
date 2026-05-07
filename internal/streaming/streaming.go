package streaming

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"syscall"
	"time"

	"github.com/the-super-company/mihomoctl/internal/render"
)

type ErrorEvent struct {
	Type  string           `json:"type"`
	Error render.ErrorBody `json:"error"`
}

func WriteErrorAndSuppress(out io.Writer, code int, errCode, msg string, details any) error {
	ce := render.NewError(code, msg, errCode, details)
	if err := WriteError(out, ce); err != nil {
		return err
	}
	ce.SuppressRender = true
	return ce
}

func WriteError(out io.Writer, err error) error {
	return WriteNDJSON(out, ErrorEvent{Type: "error", Error: render.ToErrorBody(err)})
}

func WriteNDJSON(out io.Writer, v any) error {
	if err := json.NewEncoder(out).Encode(v); err != nil {
		if IsBrokenPipe(err) {
			return render.NewError(render.ExitOK, "broken stdout pipe", "", nil)
		}
		return render.NewError(render.ExitCantOut, fmt.Sprintf("cannot write stream output: %v", err), "output_error", nil)
	}
	return nil
}

func WriteTextLine(out io.Writer, line string) error {
	if _, err := fmt.Fprintln(out, line); err != nil {
		if IsBrokenPipe(err) {
			return render.NewError(render.ExitOK, "broken stdout pipe", "", nil)
		}
		return render.NewError(render.ExitCantOut, fmt.Sprintf("cannot write stream output: %v", err), "output_error", nil)
	}
	return nil
}

func IsBrokenPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE)
}

func SleepReconnect(ctx context.Context, failures int) error {
	timer := time.NewTimer(ReconnectDelay(failures))
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func ReconnectDelay(failures int) time.Duration {
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

func SendLatest[T any](ch chan T, v T) {
	select {
	case ch <- v:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	ch <- v
}

func ConsumeLatest[T any](ctx context.Context, timeout time.Duration, read func(context.Context) (T, error), write func(T) error) (bool, error) {
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan T, 1)
	errs := make(chan error, 1)
	go func() {
		for {
			readCtx := streamCtx
			var cancelRead context.CancelFunc
			if timeout > 0 {
				readCtx, cancelRead = context.WithTimeout(streamCtx, timeout)
			}
			event, err := read(readCtx)
			if cancelRead != nil {
				cancelRead()
			}
			if err != nil {
				errs <- err
				return
			}
			SendLatest(events, event)
		}
	}()
	hadEvent := false
	for {
		select {
		case <-ctx.Done():
			return hadEvent, ctx.Err()
		case err := <-errs:
			return hadEvent, err
		case event := <-events:
			if err := write(event); err != nil {
				return hadEvent, err
			}
			hadEvent = true
		}
	}
}

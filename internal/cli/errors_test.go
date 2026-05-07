package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestUsageErrorExitCode(t *testing.T) {
	err := run([]string{"status", "extra"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, "status takes no arguments")
}

func TestController5xxExitCode(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{configCode: http.StatusInternalServerError})
	err := run([]string{"--endpoint", srv.URL, "status"}, &bytes.Buffer{})
	assertCLIError(t, err, exitTempFail, "HTTP 500")
}

func TestNetworkErrorExitCode(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	endpoint := srv.URL
	srv.Close()

	err := run([]string{"--endpoint", endpoint, "status"}, &bytes.Buffer{})
	assertCLIError(t, err, exitTempFail, "cannot connect")
}

func TestTimeoutExitCode(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{delay: 20 * time.Millisecond})
	err := run([]string{"--endpoint", srv.URL, "--timeout", "1ms", "status"}, &bytes.Buffer{})
	assertCLIError(t, err, exitTempFail, "cannot connect")
}

func TestDecodeFailureExitCode(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{configBody: "{"})
	err := run([]string{"--endpoint", srv.URL, "status"}, &bytes.Buffer{})
	assertCLIError(t, err, exitSoftware, "cannot decode")
}

func assertCLIError(t *testing.T, err error, wantCode int, wantMsg string) {
	t.Helper()
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected cliError, got %T %[1]v", err)
	}
	if ce.code != wantCode {
		t.Fatalf("exit code = %d, want %d; msg=%q", ce.code, wantCode, ce.msg)
	}
	if wantMsg != "" && !strings.Contains(ce.msg, wantMsg) {
		t.Fatalf("message %q missing %q", ce.msg, wantMsg)
	}
}

package cli

import (
	"bytes"
	"encoding/json"
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

func TestRunUsesCentralErrorRenderer(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"status", "extra"}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "status takes no arguments") {
		t.Fatalf("stderr = %q, want usage message", errOut.String())
	}
}

func TestRunJSONErrorEnvelope(t *testing.T) {
	var out, errOut bytes.Buffer
	code := Run([]string{"--json", "status", "extra"}, &out, &errOut)
	if code != exitUsage {
		t.Fatalf("exit code = %d, want %d", code, exitUsage)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	var got struct {
		Error struct {
			Code     string `json:"code"`
			Category string `json:"category"`
			Message  string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON envelope: %v\n%s", err, errOut.String())
	}
	if got.Error.Code != "usage_error" || got.Error.Category != "usage" || !strings.Contains(got.Error.Message, "status takes no arguments") {
		t.Fatalf("unexpected envelope: %+v", got.Error)
	}
}

func assertCLIError(t *testing.T, err error, wantCode int, wantMsg string) {
	t.Helper()
	var ce *cliError
	if !errors.As(err, &ce) {
		t.Fatalf("expected cliError, got %T %[1]v", err)
	}
	if ce.Code != wantCode {
		t.Fatalf("exit code = %d, want %d; msg=%q", ce.Code, wantCode, ce.Message)
	}
	if wantMsg != "" && !strings.Contains(ce.Message, wantMsg) {
		t.Fatalf("message %q missing %q", ce.Message, wantMsg)
	}
}

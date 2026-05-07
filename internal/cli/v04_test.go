package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/the-super-company/mihomoctl/internal/mihomo"
)

func TestDNSQueryJSONShapeNXDOMAINAndInvalidType(t *testing.T) {
	srv := fakeMihomo(t, "")
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "dns", "query", "example.com", "--type", "A", "--json"}, &out); err != nil {
		t.Fatalf("dns query failed: %v", err)
	}
	var got dnsOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Domain != "example.com" || got.QueryType != "A" || got.Status != "NOERROR" || len(got.Answers) != 1 || got.Answers[0].TTL != 60 {
		t.Fatalf("unexpected DNS output: %+v", got)
	}

	nxdomain := fakeMihomoWith(t, fakeOptions{dnsBody: `{"Status":3,"Question":[{"name":"missing.example.","type":1}],"Answer":[]}`})
	out.Reset()
	if err := run([]string{"--endpoint", nxdomain.URL, "dns", "query", "missing.example", "--json"}, &out); err != nil {
		t.Fatalf("NXDOMAIN should exit 0: %v", err)
	}
	var nx dnsOutput
	if err := json.Unmarshal(out.Bytes(), &nx); err != nil {
		t.Fatalf("invalid NXDOMAIN JSON: %v\n%s", err, out.String())
	}
	if nx.Status != "NXDOMAIN" || len(nx.Answers) != 0 {
		t.Fatalf("unexpected NXDOMAIN output: %+v", nx)
	}

	assertCLIError(t, run([]string{"--endpoint", srv.URL, "dns", "query", "example.com", "--type", "BOGUS"}, &bytes.Buffer{}), exitUsage, "unsupported DNS query type")
}

func TestCacheClearJSONAndPartialFailureEnvelope(t *testing.T) {
	var paths []string
	srv := fakeMihomoWith(t, fakeOptions{onCacheFlush: func(path string) {
		paths = append(paths, path)
	}})
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "cache", "clear", "all", "--json"}, &out); err != nil {
		t.Fatalf("cache clear all failed: %v", err)
	}
	var success cacheOutput
	if err := json.Unmarshal(out.Bytes(), &success); err != nil {
		t.Fatalf("invalid cache JSON: %v\n%s", err, out.String())
	}
	if success.Cache != "all" || !success.Cleared || len(success.Results) != 2 {
		t.Fatalf("unexpected cache output: %+v", success)
	}
	if strings.Join(paths, ",") != "/cache/fakeip/flush,/cache/dns/flush" {
		t.Fatalf("cache clear order = %v", paths)
	}

	partial := fakeMihomoWith(t, fakeOptions{dnsFlushCode: http.StatusInternalServerError})
	out.Reset()
	var errOut bytes.Buffer
	code := Run([]string{"--endpoint", partial.URL, "--json", "cache", "clear", "all"}, &out, &errOut)
	if code != exitTempFail {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, exitTempFail, errOut.String())
	}
	if out.Len() != 0 {
		t.Fatalf("partial failure should not emit success stdout: %s", out.String())
	}
	var env struct {
		Error struct {
			Code     string `json:"code"`
			Category string `json:"category"`
			Details  struct {
				Cache   string        `json:"cache"`
				Results []cacheResult `json:"results"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil {
		t.Fatalf("invalid error envelope: %v\n%s", err, errOut.String())
	}
	if env.Error.Code != "cache_partial_failure" || env.Error.Category != "tempfail" || env.Error.Details.Cache != "all" || len(env.Error.Details.Results) != 2 || env.Error.Details.Results[1].Cleared {
		t.Fatalf("unexpected partial failure envelope: %+v", env.Error)
	}
}

func TestConnectionsWatchAuthFailureUsesNoPermEnvelope(t *testing.T) {
	srv := fakeMihomo(t, "secret")
	var out, errOut bytes.Buffer
	code := Run([]string{"--endpoint", srv.URL, "--json", "--secret", "wrong", "connections", "watch", "--no-reconnect"}, &out, &errOut)
	if code != exitNoPerm {
		t.Fatalf("exit code = %d, want %d; stderr=%s", code, exitNoPerm, errOut.String())
	}
	var env struct {
		Error struct {
			Code     string `json:"code"`
			Category string `json:"category"`
			Message  string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &env); err != nil {
		t.Fatalf("invalid error envelope: %v\n%s", err, errOut.String())
	}
	if env.Error.Code != "auth_failed" || env.Error.Category != "noperm" || strings.Contains(env.Error.Message, "wrong") {
		t.Fatalf("unexpected auth envelope: %+v", env.Error)
	}
}

func TestConnectionsWatchJSONEventFilterAndReconnectExhausted(t *testing.T) {
	var raw struct {
		Connections []mihomo.Connection `json:"connections"`
	}
	b, err := json.Marshal(map[string]any{"connections": testConnections()})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	var out bytes.Buffer
	err = writeConnectionWatchEvent(&out, config{jsonOut: true}, connectionsWatchOptions{filter: "google.com"}, mihomo.WatchEvent{Connections: raw.Connections, ReceivedAt: time.Date(2026, 5, 7, 1, 2, 3, 0, time.UTC)})
	if err != nil {
		t.Fatalf("write event: %v", err)
	}
	var event connectionWatchEvent
	if err := json.Unmarshal(out.Bytes(), &event); err != nil {
		t.Fatalf("invalid NDJSON event: %v\n%s", err, out.String())
	}
	if event.Type != "event" || event.Data.EventAction != "snapshot" || event.Data.ReceivedAt != "2026-05-07T01:02:03Z" || len(event.Data.Connections) != 1 || event.Data.Connections[0].ID != "c-old" {
		t.Fatalf("unexpected watch event: %+v", event)
	}

	srv := httptest.NewServer(http.NewServeMux())
	endpoint := srv.URL
	srv.Close()
	out.Reset()
	client, err := newClient(config{endpoint: endpoint, timeout: time.Millisecond})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	err = runConnectionsWatch(context.Background(), &out, config{jsonOut: true, timeout: time.Millisecond}, client, connectionsWatchOptions{interval: time.Millisecond, maxReconnectAttempts: 1})
	assertCLIError(t, err, exitTempFail, "reconnect attempts exhausted")
	if !strings.Contains(out.String(), `"type":"error"`) || !strings.Contains(out.String(), `"code":"reconnect_exhausted"`) {
		t.Fatalf("stream should include reconnect_exhausted error event:\n%s", out.String())
	}
}

func TestConnectionsWatchCanceledContextExitsCleanly(t *testing.T) {
	srv := httptest.NewServer(http.NewServeMux())
	defer srv.Close()
	client, err := newClient(config{endpoint: srv.URL, timeout: time.Millisecond})
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = runConnectionsWatch(ctx, &bytes.Buffer{}, config{timeout: time.Millisecond}, client, connectionsWatchOptions{interval: time.Millisecond, noReconnect: true})
	if err != nil {
		t.Fatalf("canceled context should exit cleanly: %v", err)
	}
}

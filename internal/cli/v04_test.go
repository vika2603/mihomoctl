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

	"github.com/charmbracelet/lipgloss"
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

	qtype0 := fakeMihomoWith(t, fakeOptions{dnsBody: `{"Status":0,"Question":[{"name":"ipv6.example.","type":0}],"Answer":[]}`})
	out.Reset()
	if err := run([]string{"--endpoint", qtype0.URL, "dns", "query", "ipv6.example", "--type", "AAAA", "--json"}, &out); err != nil {
		t.Fatalf("AAAA query failed: %v", err)
	}
	var typ dnsOutput
	if err := json.Unmarshal(out.Bytes(), &typ); err != nil {
		t.Fatalf("invalid QTYPE0 JSON: %v\n%s", err, out.String())
	}
	if typ.QueryType != "AAAA" {
		t.Fatalf("query_type = %q, want AAAA", typ.QueryType)
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

func TestCacheClearBareErrorSuggestsTargets(t *testing.T) {
	err := run([]string{"cache", "clear"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, "cache clear requires a target")
	assertCLIError(t, err, exitUsage, "cache clear dns")
}

func TestCacheClearUnknownTargetSuggestsKnownTarget(t *testing.T) {
	err := run([]string{"cache", "clear", "dn"}, &bytes.Buffer{})
	assertCLIError(t, err, exitUsage, `unknown cache clear target "dn"`)
	assertCLIError(t, err, exitUsage, "Did you mean this?")
	assertCLIError(t, err, exitUsage, "dns")
}

func TestConnectionsListHumanFormatsIECBytesAndAlias(t *testing.T) {
	srv := fakeMihomoWith(t, fakeOptions{connections: []map[string]any{
		testConnection("c-large", "2026-05-07T04:00:00Z", "tcp", "192.0.2.20", "54000", "203.0.113.1", "443", "example.com", "MATCH", []string{"Proxy"}, 1536, 1048576),
	}})
	var out bytes.Buffer
	if err := run([]string{"--endpoint", srv.URL, "conns", "list"}, &out); err != nil {
		t.Fatalf("conns list failed: %v", err)
	}
	if !strings.Contains(out.String(), "1.5 KiB/1.0 MiB") {
		t.Fatalf("human output missing IEC bytes:\n%s", out.String())
	}

	out.Reset()
	if err := run([]string{"--endpoint", srv.URL, "conns", "list", "--json"}, &out); err != nil {
		t.Fatalf("conns list --json failed: %v", err)
	}
	var got connectionsOutput
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out.String())
	}
	if got.Connections[0].UploadBytes != 1536 || got.Connections[0].DownloadBytes != 1048576 {
		t.Fatalf("JSON bytes should stay int64: %+v", got.Connections[0])
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
	raw := decodeTestConnections(t)
	var out bytes.Buffer
	err := writeConnectionWatchEvent(&out, config{jsonOut: true}, connectionsWatchOptions{filter: "google.com"}, mihomo.WatchEvent{Connections: raw.Connections, ReceivedAt: time.Date(2026, 5, 7, 1, 2, 3, 0, time.UTC)})
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

func TestConnectionsWatchLimitAppliesToJSONAppendAndTUI(t *testing.T) {
	assertCLIError(t, run([]string{"conns", "watch", "--limit", "-1"}, &bytes.Buffer{}), exitUsage, "--limit must be >= 0")

	raw := decodeTestConnections(t)
	event := mihomo.WatchEvent{Connections: raw.Connections, ReceivedAt: time.Date(2026, 5, 7, 1, 2, 3, 0, time.UTC)}

	var out bytes.Buffer
	if err := writeConnectionWatchEvent(&out, config{jsonOut: true}, connectionsWatchOptions{limit: 1}, event); err != nil {
		t.Fatalf("write JSON event: %v", err)
	}
	var got connectionWatchEvent
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid NDJSON event: %v\n%s", err, out.String())
	}
	if len(got.Data.Connections) != 1 || got.Data.Connections[0].ID != "c-new" {
		t.Fatalf("JSON limit result = %+v", got.Data.Connections)
	}

	out.Reset()
	if err := writeConnectionWatchEvent(&out, config{jsonOut: true}, connectionsWatchOptions{limit: 0}, event); err != nil {
		t.Fatalf("write unlimited JSON event: %v", err)
	}
	got = connectionWatchEvent{}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid unlimited NDJSON event: %v\n%s", err, out.String())
	}
	if len(got.Data.Connections) != 3 {
		t.Fatalf("default JSON limit should be unlimited: %+v", got.Data.Connections)
	}

	out.Reset()
	if err := writeConnectionWatchEvent(&out, config{}, connectionsWatchOptions{limit: 2}, event); err != nil {
		t.Fatalf("write append event: %v", err)
	}
	if strings.Contains(out.String(), "142.250.72.14:443") || strings.Count(strings.TrimSpace(out.String()), "\n") != 2 {
		t.Fatalf("append output should include header plus two limited rows:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "300 B/400 B") || !strings.Contains(out.String(), "500 B/600 B") {
		t.Fatalf("append output missing formatted bytes:\n%s", out.String())
	}

	var emptyRaw struct {
		Connections []mihomo.Connection `json:"connections"`
	}
	emptyBytes, err := json.Marshal(map[string]any{"connections": []map[string]any{
		testConnection("c-empty", "2026-05-07T04:00:00Z", "tcp", "", "", "", "", "", "", nil, 0, 0),
	}})
	if err != nil {
		t.Fatalf("marshal empty connection fixture: %v", err)
	}
	if err := json.Unmarshal(emptyBytes, &emptyRaw); err != nil {
		t.Fatalf("unmarshal empty connection fixture: %v", err)
	}
	empty := mihomo.WatchEvent{Connections: emptyRaw.Connections, ReceivedAt: event.ReceivedAt}
	out.Reset()
	if err := writeConnectionWatchEvent(&out, config{}, connectionsWatchOptions{limit: 1}, empty); err != nil {
		t.Fatalf("write append event with empty fields: %v", err)
	}
	if !strings.Contains(out.String(), "-\t-\ttcp\t-\t-\t0 B/0 B") {
		t.Fatalf("append watch output should dash empty fields:\n%s", out.String())
	}
	emptyTUI := renderConnectionWatchTUI(connectionsWatchOptions{limit: 1}, empty, buildWatchConnectionsOutput(empty.Connections, "", 1), 80)
	// Per CEO directive msg=c414c475 ("表格不需要边框，去掉"): the TUI table must not
	// contain any vertical / horizontal box-drawing border characters.
	for _, ch := range []string{"│", "─", "┌", "┐", "└", "┘", "├", "┤", "┬", "┴", "┼"} {
		if strings.Contains(emptyTUI, ch) {
			t.Fatalf("TUI watch output should have no border characters (saw %q):\n%s", ch, emptyTUI)
		}
	}
	if !strings.Contains(emptyTUI, "2026-05-07T04:00:00Z-         -          tcp") {
		t.Fatalf("TUI watch output should dash empty fields without borders:\n%s", emptyTUI)
	}

	tuiResult := buildWatchConnectionsOutput(event.Connections, "cloudflare", 1)
	tui := renderConnectionWatchTUI(connectionsWatchOptions{limit: 1, filter: "cloudflare"}, event, tuiResult, 0)
	for _, want := range []string{"mihomoctl connections watch", "matches: 1", "filter: cloudflare", "300 B/400 B"} {
		if !strings.Contains(tui, want) {
			t.Fatalf("TUI output missing %q:\n%s", want, tui)
		}
	}

	limitedResult := buildWatchConnectionsOutput(event.Connections, "", 1)
	limitedTUI := renderConnectionWatchTUI(connectionsWatchOptions{limit: 1}, event, limitedResult, 80)
	for _, want := range []string{"matches: 3", "shown: 1"} {
		if !strings.Contains(limitedTUI, want) {
			t.Fatalf("limited TUI output missing %q:\n%s", want, limitedTUI)
		}
	}

	narrow := renderConnectionWatchTUI(connectionsWatchOptions{limit: 1}, event, limitedResult, 59)
	if !strings.Contains(narrow, "id") || strings.Contains(narrow, "started_at") || !strings.Contains(narrow, "c-new") {
		t.Fatalf("narrow TUI output should use key columns only:\n%s", narrow)
	}

	long := raw
	long.Connections[0].Metadata.SourceIP = "192.0.2.123"
	long.Connections[0].Metadata.DestinationIP = "203.0.113.123"
	long.Connections[0].Metadata.Host = strings.Repeat("very-long-hostname-", 5) + "example.com"
	long.Connections[0].Rule = strings.Repeat("DOMAIN-SUFFIX,example.com,", 4)
	long.Connections[0].Chains = []string{strings.Repeat("VeryLongNodeName", 6)}
	longEvent := mihomo.WatchEvent{Connections: long.Connections, ReceivedAt: event.ReceivedAt}
	longTUI := renderConnectionWatchTUI(connectionsWatchOptions{limit: 1}, longEvent, buildWatchConnectionsOutput(long.Connections, "", 1), 80)
	assertMaxRenderedWidth(t, longTUI, 80)
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

func decodeTestConnections(t *testing.T) struct {
	Connections []mihomo.Connection `json:"connections"`
} {
	t.Helper()
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
	return raw
}

func assertMaxRenderedWidth(t *testing.T, text string, max int) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if got := lipgloss.Width(line); got > max {
			t.Fatalf("rendered line width = %d, want <= %d:\n%s\n\nfull output:\n%s", got, max, line, text)
		}
	}
}

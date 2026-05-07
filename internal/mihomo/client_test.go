package mihomo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestQueryDNSBuildsRequest(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dns/query" {
			t.Fatalf("path = %q, want /dns/query", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "example.com" {
			t.Fatalf("name query = %q", r.URL.Query().Get("name"))
		}
		if r.URL.Query().Get("type") != "AAAA" {
			t.Fatalf("type query = %q", r.URL.Query().Get("type"))
		}
		sawAuth = r.Header.Get("Authorization") == "Bearer secret"
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Status":0,"Answer":[{"name":"example.com.","type":28,"TTL":60,"data":"2001:db8::1"}]}`))
	}))
	defer srv.Close()

	client, err := New(srv.URL, "secret", time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := client.QueryDNS(context.Background(), "example.com", "AAAA")
	if err != nil {
		t.Fatalf("QueryDNS: %v", err)
	}
	if !sawAuth {
		t.Fatal("Authorization header was not sent")
	}
	if got.Status != 0 || len(got.Answers) != 1 || got.Answers[0].Data != "2001:db8::1" {
		t.Fatalf("unexpected DNS response: %+v", got)
	}
}

func TestCacheFlushMethods(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client, err := New(srv.URL, "", time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := client.FlushFakeIPCache(context.Background()); err != nil {
		t.Fatalf("FlushFakeIPCache: %v", err)
	}
	if err := client.FlushDNSCache(context.Background()); err != nil {
		t.Fatalf("FlushDNSCache: %v", err)
	}
	if err := client.ClearCache(context.Background()); err != nil {
		t.Fatalf("ClearCache: %v", err)
	}

	want := []string{"/cache/fakeip/flush", "/cache/dns/flush", "/cache/fakeip/flush", "/cache/dns/flush"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %#v, want %#v", paths, want)
	}
}

func TestWatchConnectionsUsesHeaderAuthAndInterval(t *testing.T) {
	var sawAuth bool
	var sawToken bool
	var sawInterval string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/connections" {
			t.Fatalf("path = %q, want /connections", r.URL.Path)
		}
		sawAuth = r.Header.Get("Authorization") == "Bearer secret"
		sawToken = r.URL.Query().Get("token") != ""
		sawInterval = r.URL.Query().Get("interval")
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Fatalf("accept websocket: %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		err = conn.Write(r.Context(), websocket.MessageText, []byte(`{"connections":[{"id":"c1","metadata":{"network":"tcp","sourceIP":"192.0.2.1","sourcePort":"50000","destinationIP":"198.51.100.1","destinationPort":"443","host":"example.com"},"upload":1,"download":2,"start":"2026-05-07T01:00:00Z","chains":["Proxy"],"rule":"MATCH"}]}`))
		if err != nil {
			t.Fatalf("write websocket: %v", err)
		}
	}))
	defer srv.Close()

	client, err := New(srv.URL, "secret", time.Second)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	watch, err := client.WatchConnections(context.Background(), ConnectionsWatchOptions{Interval: 1500 * time.Millisecond})
	if err != nil {
		t.Fatalf("WatchConnections: %v", err)
	}
	defer watch.Close()
	event, err := watch.Read(context.Background())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !sawAuth {
		t.Fatal("Authorization header was not sent")
	}
	if sawToken {
		t.Fatal("websocket URL must not include token query")
	}
	if sawInterval != "1500" {
		t.Fatalf("interval query = %q, want 1500", sawInterval)
	}
	if len(event.Connections) != 1 || event.Connections[0].ID != "c1" {
		t.Fatalf("unexpected watch event: %+v", event)
	}
}

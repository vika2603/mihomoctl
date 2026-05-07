package mihomo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
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

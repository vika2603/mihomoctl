package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"
)

type fakeOptions struct {
	secret          string
	configCode      int
	configBody      string
	versionCode     int
	proxyCode       int
	proxyBody       string
	groupCode       int
	groupBody       string
	proxies         map[string]any
	groupDelays     map[string]map[string]int
	onGroupDelay    func(group string, query url.Values)
	connectionsCode int
	connectionsBody string
	connections     []map[string]any
	rulesCode       int
	rulesBody       string
	rules           []map[string]any
	providersCode   int
	providersBody   string
	providers       map[string]any
	ruleProviders   map[string]any
	proxyDelays     map[string]int
	onProxyDelay    func(proxy string, query url.Values)
	healthcheckCode int
	onHealthcheck   func(provider string)
	dnsCode         int
	dnsBody         string
	fakeIPFlushCode int
	dnsFlushCode    int
	onCacheFlush    func(path string)
	delay           time.Duration
	onProxySet      func(uri string)
}

func fakeMihomo(t testingT, secret string) *httptest.Server {
	return fakeMihomoWith(t, fakeOptions{secret: secret})
}

type testingT interface {
	Helper()
	Fatalf(string, ...any)
}

func fakeMihomoWith(t testingT, opts fakeOptions) *httptest.Server {
	t.Helper()
	if opts.configCode == 0 {
		opts.configCode = http.StatusOK
	}
	if opts.versionCode == 0 {
		opts.versionCode = http.StatusOK
	}
	if opts.proxyCode == 0 {
		opts.proxyCode = http.StatusOK
	}
	if opts.groupCode == 0 {
		opts.groupCode = http.StatusOK
	}
	if opts.connectionsCode == 0 {
		opts.connectionsCode = http.StatusOK
	}
	if opts.rulesCode == 0 {
		opts.rulesCode = http.StatusOK
	}
	if opts.providersCode == 0 {
		opts.providersCode = http.StatusOK
	}
	if opts.healthcheckCode == 0 {
		opts.healthcheckCode = http.StatusNoContent
	}
	if opts.dnsCode == 0 {
		opts.dnsCode = http.StatusOK
	}
	if opts.fakeIPFlushCode == 0 {
		opts.fakeIPFlushCode = http.StatusNoContent
	}
	if opts.dnsFlushCode == 0 {
		opts.dnsFlushCode = http.StatusNoContent
	}
	if opts.proxies == nil {
		opts.proxies = map[string]any{
			"Proxy":     map[string]any{"name": "Proxy", "type": "Selector", "now": "A", "all": []string{"A", "B"}},
			"Auto / 香港": map[string]any{"name": "Auto / 香港", "type": "Selector", "now": "A", "all": []string{"A", "B"}},
			"A":         map[string]any{"name": "A", "type": "Shadowsocks"},
		}
	}
	if opts.groupDelays == nil {
		opts.groupDelays = map[string]map[string]int{"Proxy": {"A": 20, "B": 10}}
	}
	if opts.rules == nil {
		opts.rules = testRules()
	}
	if opts.providers == nil {
		opts.providers = testProviders()
	}
	if opts.ruleProviders == nil {
		opts.ruleProviders = testRuleProviders()
	}
	if opts.proxyDelays == nil {
		opts.proxyDelays = map[string]int{"A": 16, "B": 42}
	}

	mux := http.NewServeMux()
	requireAuth := func(w http.ResponseWriter, r *http.Request) bool {
		if opts.secret == "" {
			return true
		}
		if r.Header.Get("Authorization") != "Bearer "+opts.secret {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
		return true
	}
	maybeDelay := func() {
		if opts.delay > 0 {
			time.Sleep(opts.delay)
		}
	}
	mux.HandleFunc("/configs", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.configCode)
		if opts.configBody != "" {
			_, _ = w.Write([]byte(opts.configBody))
			return
		}
		if opts.configCode >= 200 && opts.configCode < 300 {
			writeTestJSON(t, w, map[string]string{"mode": "rule"})
		}
	})
	mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		w.WriteHeader(opts.versionCode)
		if opts.versionCode >= 200 && opts.versionCode < 300 {
			writeTestJSON(t, w, map[string]string{"version": "v-test"})
		}
	})
	mux.HandleFunc("/proxies", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		w.WriteHeader(opts.proxyCode)
		if opts.proxyBody != "" {
			_, _ = w.Write([]byte(opts.proxyBody))
			return
		}
		if opts.proxyCode >= 200 && opts.proxyCode < 300 {
			writeTestJSON(t, w, map[string]any{"proxies": opts.proxies})
		}
	})
	mux.HandleFunc("/proxies/", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/delay") {
			proxyPath := strings.TrimPrefix(r.URL.Path, "/proxies/")
			proxyPath = strings.TrimSuffix(proxyPath, "/delay")
			proxy, err := url.PathUnescape(proxyPath)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			delay, ok := opts.proxyDelays[proxy]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			if opts.onProxyDelay != nil {
				opts.onProxyDelay(proxy, r.URL.Query())
			}
			writeTestJSON(t, w, map[string]int{"delay": delay})
			return
		}
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.onProxySet != nil {
			opts.onProxySet(r.RequestURI)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/group", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.groupCode)
		if opts.groupBody != "" {
			_, _ = w.Write([]byte(opts.groupBody))
			return
		}
		if opts.groupCode >= 200 && opts.groupCode < 300 {
			writeTestJSON(t, w, map[string]any{"proxies": groupListFromProxies(opts.proxies)})
		}
	})
	mux.HandleFunc("/group/", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		groupPath := strings.TrimPrefix(r.URL.Path, "/group/")
		if !strings.HasSuffix(r.URL.Path, "/delay") {
			group, err := url.PathUnescape(groupPath)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			proxy, ok := proxyFromFake(opts.proxies, group)
			if !ok || len(toStringSlice(proxy["all"])) == 0 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			writeTestJSON(t, w, proxy)
			return
		}
		groupPath = strings.TrimSuffix(groupPath, "/delay")
		group, err := url.PathUnescape(groupPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if opts.onGroupDelay != nil {
			opts.onGroupDelay(group, r.URL.Query())
		}
		delays, ok := opts.groupDelays[group]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeTestJSON(t, w, delays)
	})
	mux.HandleFunc("/connections", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.connectionsCode)
		if opts.connectionsBody != "" {
			_, _ = w.Write([]byte(opts.connectionsBody))
			return
		}
		if opts.connectionsCode >= 200 && opts.connectionsCode < 300 {
			connections := opts.connections
			if connections == nil {
				connections = testConnections()
			}
			writeTestJSON(t, w, map[string]any{"connections": connections})
		}
	})
	mux.HandleFunc("/rules", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.rulesCode)
		if opts.rulesBody != "" {
			_, _ = w.Write([]byte(opts.rulesBody))
			return
		}
		if opts.rulesCode >= 200 && opts.rulesCode < 300 {
			writeTestJSON(t, w, map[string]any{"rules": opts.rules})
		}
	})
	mux.HandleFunc("/providers/proxies", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.providersCode)
		if opts.providersBody != "" {
			_, _ = w.Write([]byte(opts.providersBody))
			return
		}
		if opts.providersCode >= 200 && opts.providersCode < 300 {
			writeTestJSON(t, w, map[string]any{"providers": opts.providers})
		}
	})
	mux.HandleFunc("/providers/proxies/", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		providerPath := strings.TrimPrefix(r.URL.Path, "/providers/proxies/")
		healthcheck := strings.HasSuffix(providerPath, "/healthcheck")
		if healthcheck {
			providerPath = strings.TrimSuffix(providerPath, "/healthcheck")
		}
		provider, err := url.PathUnescape(providerPath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if healthcheck {
			if opts.onHealthcheck != nil {
				opts.onHealthcheck(provider)
			}
			w.WriteHeader(opts.healthcheckCode)
			return
		}
		p, ok := opts.providers[provider]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeTestJSON(t, w, p)
	})
	mux.HandleFunc("/providers/rules", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeTestJSON(t, w, map[string]any{"providers": opts.ruleProviders})
	})
	mux.HandleFunc("/dns/query", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(opts.dnsCode)
		if opts.dnsBody != "" {
			_, _ = w.Write([]byte(opts.dnsBody))
			return
		}
		if opts.dnsCode >= 200 && opts.dnsCode < 300 {
			writeTestJSON(t, w, map[string]any{
				"Status":   0,
				"Question": []map[string]any{{"name": r.URL.Query().Get("name") + ".", "type": 1}},
				"Answer":   []map[string]any{{"name": r.URL.Query().Get("name") + ".", "type": 1, "TTL": 60, "data": "198.51.100.10"}},
			})
		}
	})
	mux.HandleFunc("/cache/fakeip/flush", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.onCacheFlush != nil {
			opts.onCacheFlush(r.URL.Path)
		}
		w.WriteHeader(opts.fakeIPFlushCode)
	})
	mux.HandleFunc("/cache/dns/flush", func(w http.ResponseWriter, r *http.Request) {
		maybeDelay()
		if !requireAuth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if opts.onCacheFlush != nil {
			opts.onCacheFlush(r.URL.Path)
		}
		w.WriteHeader(opts.dnsFlushCode)
	})
	return httptest.NewServer(mux)
}

func groupListFromProxies(proxies map[string]any) []map[string]any {
	groups := make([]map[string]any, 0)
	for name := range proxies {
		proxy, ok := proxyFromFake(proxies, name)
		if !ok || len(toStringSlice(proxy["all"])) == 0 {
			continue
		}
		groups = append(groups, proxy)
	}
	return groups
}

func proxyFromFake(proxies map[string]any, name string) (map[string]any, bool) {
	v, ok := proxies[name]
	if !ok {
		return nil, false
	}
	proxy, ok := v.(map[string]any)
	if !ok {
		return nil, false
	}
	if proxy["name"] == nil || proxy["name"] == "" {
		proxy = cloneMap(proxy)
		proxy["name"] = name
	}
	return proxy, true
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func toStringSlice(v any) []string {
	items, ok := v.([]string)
	if ok {
		return items
	}
	return nil
}

func writeTestJSON(t testingT, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func testConnections() []map[string]any {
	return []map[string]any{
		testConnection("c-old", "2026-05-07T01:00:00Z", "tcp", "192.0.2.10", "51000", "142.250.72.14", "443", "google.com", "MATCH", []string{"Proxy", "A"}, 100, 200),
		testConnection("c-new", "2026-05-07T03:00:00Z", "tcp", "192.0.2.11", "52000", "1.1.1.1", "443", "cloudflare.com", "DOMAIN-SUFFIX", []string{"Proxy", "B"}, 300, 400),
		testConnection("c-mid", "2026-05-07T02:00:00Z", "udp", "198.51.100.9", "53000", "8.8.8.8", "53", "dns.google", "GEOIP", []string{"DNS"}, 500, 600),
	}
}

func testConnection(id, start, network, srcIP, srcPort, dstIP, dstPort, host, rule string, chains []string, upload, download int64) map[string]any {
	return map[string]any{
		"id": id,
		"metadata": map[string]any{
			"network":         network,
			"sourceIP":        srcIP,
			"sourcePort":      srcPort,
			"destinationIP":   dstIP,
			"destinationPort": dstPort,
			"host":            host,
		},
		"upload":   upload,
		"download": download,
		"start":    start,
		"chains":   chains,
		"rule":     rule,
	}
}

func testRules() []map[string]any {
	return []map[string]any{
		{"index": 3, "type": "IP-CIDR", "payload": "1.1.1.0/24", "proxy": "PROXY"},
		{"index": 1, "type": "DOMAIN-SUFFIX", "payload": "google.com", "proxy": "PROXY"},
		{"index": 2, "type": "DOMAIN", "payload": "example.com", "proxy": "DIRECT"},
		{"index": 4, "type": "MATCH", "payload": "", "proxy": "FALLBACK"},
		{"index": 5, "type": "DOMAIN-KEYWORD", "payload": "google", "proxy": "PROXY"},
	}
}

func testProviders() map[string]any {
	return map[string]any{
		"airport": map[string]any{
			"name":        "airport",
			"type":        "Proxy",
			"vehicleType": "HTTP",
			"updatedAt":   "2026-05-07T04:00:00Z",
			"proxies": []map[string]any{
				{"name": "A", "alive": true},
				{"name": "B", "alive": false},
			},
		},
		"empty": map[string]any{
			"name":        "empty",
			"type":        "Proxy",
			"vehicleType": "File",
			"updatedAt":   "2026-05-07T03:00:00Z",
			"proxies":     []map[string]any{},
		},
	}
}

func testRuleProviders() map[string]any {
	return map[string]any{
		"geoip": map[string]any{
			"name":        "geoip",
			"type":        "Rule",
			"vehicleType": "HTTP",
			"behavior":    "IPCIDR",
			"ruleCount":   1024,
			"updatedAt":   "2026-05-07T05:00:00Z",
		},
		"rejects": map[string]any{
			"name":        "rejects",
			"type":        "Rule",
			"vehicleType": "File",
			"behavior":    "Domain",
			"ruleCount":   3,
			"updatedAt":   "2026-05-07T04:30:00Z",
		},
	}
}

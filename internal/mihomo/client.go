package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type ErrorKind int

const (
	ErrAuth ErrorKind = iota + 1
	ErrBadRequest
	ErrNotFound
	ErrUnavailable
	ErrSoftware
)

type Error struct {
	Kind ErrorKind
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

type Client struct {
	base    *url.URL
	secret  string
	http    *http.Client
	timeout time.Duration
}

func New(endpoint, secret string, timeout time.Duration) (*Client, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, fmt.Errorf("endpoint cannot be empty")
	}
	base, err := url.Parse(endpoint)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return nil, fmt.Errorf("invalid endpoint %q", endpoint)
	}
	return &Client{
		base:    base,
		secret:  secret,
		http:    &http.Client{},
		timeout: timeout,
	}, nil
}

func (c *Client) Endpoint() string {
	return c.base.String()
}

func (c *Client) GetConfigs(ctx context.Context) (Config, error) {
	var v Config
	err := c.do(ctx, http.MethodGet, "/configs", nil, &v)
	return v, err
}

func (c *Client) PatchMode(ctx context.Context, mode string) error {
	return c.do(ctx, http.MethodPatch, "/configs", map[string]string{"mode": mode}, nil)
}

func (c *Client) Version(ctx context.Context) (Version, error) {
	var v Version
	err := c.do(ctx, http.MethodGet, "/version", nil, &v)
	return v, err
}

func (c *Client) ListProxies(ctx context.Context) (map[string]Proxy, error) {
	var v struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	err := c.do(ctx, http.MethodGet, "/proxies", nil, &v)
	return v.Proxies, err
}

func (c *Client) SelectProxy(ctx context.Context, group, node string) error {
	path := "/proxies/" + url.PathEscape(group)
	return c.do(ctx, http.MethodPut, path, map[string]string{"name": node}, nil)
}

type GroupDelayOptions struct {
	URL            string
	DelayTimeout   time.Duration
	RequestTimeout time.Duration
}

func (c *Client) GroupDelay(ctx context.Context, group string, opts GroupDelayOptions) (map[string]int, error) {
	values := url.Values{}
	if opts.URL != "" {
		values.Set("url", opts.URL)
	}
	if opts.DelayTimeout > 0 {
		values.Set("timeout", fmt.Sprintf("%d", opts.DelayTimeout.Milliseconds()))
	}
	path := "/group/" + url.PathEscape(group) + "/delay"
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var v map[string]int
	err := c.doWithTimeout(ctx, http.MethodGet, path, nil, &v, opts.RequestTimeout)
	return v, err
}

func (c *Client) ListConnections(ctx context.Context) (ConnectionsSnapshot, error) {
	var v ConnectionsSnapshot
	err := c.do(ctx, http.MethodGet, "/connections", nil, &v)
	return v, err
}

func (c *Client) ListRules(ctx context.Context) ([]Rule, error) {
	var v struct {
		Rules []Rule `json:"rules"`
	}
	err := c.do(ctx, http.MethodGet, "/rules", nil, &v)
	return v.Rules, err
}

func (c *Client) ListProxyProviders(ctx context.Context) (map[string]ProxyProvider, error) {
	var v struct {
		Providers map[string]ProxyProvider `json:"providers"`
	}
	err := c.do(ctx, http.MethodGet, "/providers/proxies", nil, &v)
	return v.Providers, err
}

func (c *Client) HealthcheckProxyProvider(ctx context.Context, name string) error {
	path := "/providers/proxies/" + url.PathEscape(name) + "/healthcheck"
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

func (c *Client) QueryDNS(ctx context.Context, domain, queryType string) (DNSResponse, error) {
	values := url.Values{}
	values.Set("name", domain)
	if queryType != "" {
		values.Set("type", queryType)
	}
	var v DNSResponse
	err := c.do(ctx, http.MethodGet, "/dns/query?"+values.Encode(), nil, &v)
	return v, err
}

func (c *Client) FlushFakeIPCache(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/cache/fakeip/flush", nil, nil)
}

func (c *Client) FlushDNSCache(ctx context.Context) error {
	return c.do(ctx, http.MethodPost, "/cache/dns/flush", nil, nil)
}

func (c *Client) ClearCache(ctx context.Context) error {
	if err := c.FlushFakeIPCache(ctx); err != nil {
		return err
	}
	return c.FlushDNSCache(ctx)
}

type ConnectionsWatchOptions struct {
	Interval time.Duration
}

type ConnectionsWatch struct {
	conn *websocket.Conn
}

type WatchEvent struct {
	Connections []Connection
	ReceivedAt  time.Time
}

func (c *Client) WatchConnections(ctx context.Context, opts ConnectionsWatchOptions) (*ConnectionsWatch, error) {
	u := *c.base
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return nil, &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot build websocket URL from endpoint scheme %q", c.base.Scheme)}
	}
	u.Path = strings.TrimRight(c.base.EscapedPath(), "/") + "/connections"
	values := u.Query()
	if opts.Interval > 0 {
		values.Set("interval", fmt.Sprintf("%d", opts.Interval.Milliseconds()))
	}
	u.RawQuery = values.Encode()

	headers := http.Header{}
	if c.secret != "" {
		headers.Set("Authorization", "Bearer "+c.secret)
	}
	conn, resp, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden:
				return nil, &Error{Kind: ErrAuth, Msg: "missing/invalid secret; set MIHOMOCTL_SECRET, or use --secret <value> if you accept the leak risk"}
			case http.StatusBadRequest:
				return nil, &Error{Kind: ErrBadRequest, Msg: "mihomo controller rejected the websocket request"}
			case http.StatusNotFound:
				return nil, &Error{Kind: ErrNotFound, Msg: "mihomo endpoint or requested resource not found"}
			}
			if resp.StatusCode >= 500 {
				return nil, &Error{Kind: ErrUnavailable, Msg: fmt.Sprintf("mihomo controller returned HTTP %d", resp.StatusCode)}
			}
		}
		return nil, &Error{Kind: ErrUnavailable, Msg: fmt.Sprintf("cannot connect to mihomo websocket at %s: %v", u.String(), err)}
	}
	conn.SetReadLimit(16 << 20)
	return &ConnectionsWatch{conn: conn}, nil
}

func (w *ConnectionsWatch) Read(ctx context.Context) (WatchEvent, error) {
	_, data, err := w.conn.Read(ctx)
	if err != nil {
		return WatchEvent{}, err
	}
	var snapshot ConnectionsSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return WatchEvent{}, &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot decode mihomo websocket event: %v", err)}
	}
	return WatchEvent{Connections: snapshot.Connections, ReceivedAt: time.Now().UTC()}, nil
}

func (w *ConnectionsWatch) Close() error {
	return w.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	return c.doWithTimeout(ctx, method, path, body, out, c.timeout)
}

func (c *Client) doWithTimeout(ctx context.Context, method, path string, body any, out any, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	pathOnly, rawQuery, _ := strings.Cut(path, "?")
	escapedPath := strings.TrimRight(c.base.EscapedPath(), "/") + pathOnly
	unescapedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot build request path: %v", err)}
	}
	u := *c.base
	u.Path = unescapedPath
	u.RawPath = escapedPath
	u.RawQuery = rawQuery
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot encode request: %v", err)}
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot build request: %v", err)}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.secret)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &Error{Kind: ErrUnavailable, Msg: fmt.Sprintf("cannot connect to mihomo at %s; check --endpoint or whether external-controller is enabled: %v", c.base.String(), err)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &Error{Kind: ErrAuth, Msg: "missing/invalid secret; set MIHOMOCTL_SECRET, or use --secret <value> if you accept the leak risk"}
	}
	if resp.StatusCode == http.StatusBadRequest {
		return &Error{Kind: ErrBadRequest, Msg: controllerErrorMessage(resp.Body, "mihomo controller rejected the request")}
	}
	if resp.StatusCode == http.StatusNotFound {
		return &Error{Kind: ErrNotFound, Msg: "mihomo endpoint or requested resource not found"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &Error{Kind: ErrUnavailable, Msg: fmt.Sprintf("mihomo controller returned HTTP %d", resp.StatusCode)}
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return &Error{Kind: ErrSoftware, Msg: fmt.Sprintf("cannot decode mihomo response: %v", err)}
	}
	return nil
}

func controllerErrorMessage(r io.Reader, fallback string) string {
	var v struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r).Decode(&v); err == nil && strings.TrimSpace(v.Message) != "" {
		return v.Message
	}
	return fallback
}

type Config struct {
	Mode string `json:"mode"`
}

type Version struct {
	Version string `json:"version"`
}

type Proxy struct {
	Name string   `json:"name"`
	Type string   `json:"type"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}

type ConnectionsSnapshot struct {
	Connections []Connection `json:"connections"`
}

type Connection struct {
	ID            string             `json:"id"`
	Metadata      ConnectionMetadata `json:"metadata"`
	UploadBytes   int64              `json:"upload"`
	DownloadBytes int64              `json:"download"`
	Start         time.Time          `json:"start"`
	Chains        []string           `json:"chains"`
	Rule          string             `json:"rule"`
}

type ConnectionMetadata struct {
	Network         string `json:"network"`
	SourceIP        string `json:"sourceIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationIP   string `json:"destinationIP"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
}

type Rule struct {
	Index   int    `json:"index"`
	Type    string `json:"type"`
	Payload string `json:"payload"`
	Proxy   string `json:"proxy"`
}

type ProxyProvider struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	VehicleType string          `json:"vehicleType"`
	Proxies     []ProviderProxy `json:"proxies"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type ProviderProxy struct {
	Name  string `json:"name"`
	Alive bool   `json:"alive"`
}

type DNSResponse struct {
	Status     int         `json:"Status"`
	Questions  []DNSRecord `json:"Question"`
	Answers    []DNSRecord `json:"Answer"`
	Authority  []DNSRecord `json:"Authority"`
	Additional []DNSRecord `json:"Additional"`
}

type DNSRecord struct {
	Name string `json:"name"`
	Type uint16 `json:"type"`
	TTL  uint32 `json:"TTL"`
	Data string `json:"data"`
}

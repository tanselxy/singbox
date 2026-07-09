// Package agent defines the master↔node control protocol and the client the
// master uses to drive a node. A node exposes these endpoints under /agent/,
// authenticated by a bearer token. TLS is used for encryption only (nodes have
// self-signed certs), so the client skips certificate verification and relies
// on the token for authentication.
package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tanselxy/singbox/internal/metrics"
	"github.com/tanselxy/singbox/internal/model"
)

// Path constants for the agent endpoints.
const (
	PathServer  = "/agent/server"
	PathApply   = "/agent/apply"
	PathTraffic = "/agent/traffic"
	PathMetrics = "/agent/metrics"
)

// TrafficDelta is a per-user up/down delta since the last poll.
type TrafficDelta struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// ApplyRequest is the desired client set pushed to a node.
type ApplyRequest struct {
	Clients []model.Client `json:"clients"`
}

// Client talks to one node's agent API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

// New builds an agent client for a node base URL (e.g. https://ip:port).
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http: &http.Client{
			Timeout:   10 * time.Second,
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}
}

// FetchServer returns the node's server settings (for link generation).
func (c *Client) FetchServer(ctx context.Context) (model.Server, error) {
	var srv model.Server
	err := c.do(ctx, http.MethodGet, PathServer, nil, &srv)
	return srv, err
}

// Apply pushes the desired client set to the node.
func (c *Client) Apply(ctx context.Context, clients []model.Client) error {
	return c.do(ctx, http.MethodPost, PathApply, ApplyRequest{Clients: clients}, nil)
}

// Traffic returns per-user traffic deltas since the previous call (the node
// resets its counters on read).
func (c *Client) Traffic(ctx context.Context) (map[string]TrafficDelta, error) {
	out := map[string]TrafficDelta{}
	err := c.do(ctx, http.MethodGet, PathTraffic, nil, &out)
	return out, err
}

// Metrics returns the node's current system metrics.
func (c *Client) Metrics(ctx context.Context) (metrics.System, error) {
	var m metrics.System
	err := c.do(ctx, http.MethodGet, PathMetrics, nil, &m)
	return m, err
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(msg)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

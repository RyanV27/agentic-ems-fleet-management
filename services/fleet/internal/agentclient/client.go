// Package agentclient is Go's caller of the TS Mastra triage agent
// (ARCHITECTURE.md §6 boundary 2): a single POST /triage call fired from a
// goroutine with a timeout. Down or slow, the call simply stays ESCALATED
// and visible for manual dispatch — intake (internal/intake) is never
// blocked on it and never retries here; that is the agent's own concern.
package agentclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TriageRequest is the body of POST /triage (ARCHITECTURE.md §6 boundary 2).
type TriageRequest struct {
	CallID  string `json:"callId"`
	Reason  string `json:"reason"`
	Attempt int    `json:"attempt"`
}

// Client is the sole caller of the TS agent's /triage endpoint.
type Client struct {
	HTTPClient *http.Client
	BaseURL    string
	Timeout    time.Duration
}

// NewClient builds a Client with a sane default timeout.
func NewClient(baseURL string) *Client {
	return &Client{
		HTTPClient: http.DefaultClient,
		BaseURL:    baseURL,
		Timeout:    10 * time.Second,
	}
}

// Triage posts req to the agent's /triage endpoint. Any error — timeout,
// connection refused, non-2xx status — is returned identically: the caller
// treats them all the same way (the call stays ESCALATED, REL-2).
func (c *Client) Triage(ctx context.Context, req TriageRequest) error {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("agentclient: marshal triage request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/triage", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("agentclient: build triage request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("agentclient: triage request for call %s: %w", req.CallID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("agentclient: triage for call %s returned status %d", req.CallID, resp.StatusCode)
	}
	return nil
}

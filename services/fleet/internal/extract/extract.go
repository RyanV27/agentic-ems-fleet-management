// Package extract calls OpenRouter to turn a raw 911 transcript into a
// domain.Extraction (ARCHITECTURE.md §6 boundary 1, ROADMAP.md S4). This is
// the only LLM call on the intake path (CLAUDE.md hard rule 3) and the only
// place a transcript is ever sent to a model.
//
// Extract never returns a non-nil error for a call-time failure — a timeout,
// HTTP error, rate limit, or a schema-invalid response all become a
// domain.Extraction with Confidence 0 and a populated FailureReason, so the
// rules engine escalates instead of the caller having to special-case an
// error (SEC-6, ROADMAP.md S4 c3).
package extract

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/config"
	"github.com/RyanV27/agentic-ems-fleet-management/services/fleet/internal/domain"
)

const defaultBaseURL = "https://openrouter.ai/api/v1"

// Client calls OpenRouter's chat-completions endpoint for extraction.
type Client struct {
	HTTPClient   *http.Client
	APIKey       string
	Model        string
	BaseURL      string
	ValidZoneIDs []string

	// Timeout bounds a single attempt (ARCHITECTURE.md boundary 1's "hard
	// timeout"). RetryBackoff is the delay before the one retry on a 5xx or
	// timeout (ROADMAP.md S4 c4); tests set it near zero.
	Timeout      time.Duration
	RetryBackoff time.Duration
}

// NewClient builds a Client from config and the current valid zone ids
// (from the store, per S4's scope note: extract does not read the store
// itself — the caller supplies the list).
func NewClient(cfg config.Config, validZoneIDs []string) *Client {
	return &Client{
		HTTPClient:   http.DefaultClient,
		APIKey:       cfg.OpenRouterAPIKey,
		Model:        cfg.OpenRouterExtractionModel,
		BaseURL:      defaultBaseURL,
		ValidZoneIDs: validZoneIDs,
		Timeout:      30 * time.Second,
		RetryBackoff: 500 * time.Millisecond,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string         `json:"model"`
	Temperature    float64        `json:"temperature"`
	Messages       []chatMessage  `json:"messages"`
	ResponseFormat responseFormat `json:"response_format"`
	Reasoning      reasoningSpec  `json:"reasoning"`
}

// reasoningSpec explicitly disables extended reasoning. Some OpenRouter
// models reason by default even when not asked, which both slows structured
// extraction and makes free-tier models more prone to stalling out entirely.
type reasoningSpec struct {
	Enabled bool `json:"enabled"`
}

type responseFormat struct {
	Type       string         `json:"type"`
	JSONSchema jsonSchemaSpec `json:"json_schema"`
}

type jsonSchemaSpec struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// attemptResult carries either a validated rawExtraction or a failure reason
// (and whether that failure is retryable) out of a single HTTP attempt.
type attemptResult struct {
	raw            rawExtraction
	rawResponse    string
	promptToks     int
	completionToks int
	failure        string
	retryable      bool
}

// Extract sends the transcript to OpenRouter and returns the validated
// extraction, or a zero-confidence extraction with FailureReason set.
func (c *Client) Extract(ctx context.Context, transcript string) (domain.Extraction, error) {
	start := time.Now()

	result := c.attempt(ctx, transcript)
	if result.failure != "" && result.retryable {
		time.Sleep(c.RetryBackoff)
		result = c.attempt(ctx, transcript)
	}

	latency := time.Since(start)

	if result.failure != "" {
		return domain.Extraction{
			FailureReason:    result.failure,
			Model:            c.Model,
			LatencyMs:        latency.Milliseconds(),
			PromptTokens:     result.promptToks,
			CompletionTokens: result.completionToks,
			RawResponse:      result.rawResponse,
			CreatedAt:        start,
		}, nil
	}

	return domain.Extraction{
		IncidentType:     result.raw.IncidentType,
		Severity:         result.raw.Severity,
		ZoneID:           result.raw.ZoneID,
		Keywords:         result.raw.Keywords,
		NeedsTransport:   result.raw.NeedsTransport,
		Confidence:       result.raw.Confidence,
		Model:            c.Model,
		LatencyMs:        latency.Milliseconds(),
		PromptTokens:     result.promptToks,
		CompletionTokens: result.completionToks,
		RawResponse:      result.rawResponse,
		CreatedAt:        start,
	}, nil
}

// attempt makes a single HTTP round trip and validates the response. It
// never returns a Go error — every failure mode is reported as a
// FailureReason so Extract can decide whether to retry.
func (c *Client) attempt(ctx context.Context, transcript string) attemptResult {
	attemptCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	reqBody := chatRequest{
		Model:       c.Model,
		Temperature: 0,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: userPrompt(transcript, c.ValidZoneIDs)},
		},
		ResponseFormat: responseFormat{
			Type: "json_schema",
			JSONSchema: jsonSchemaSpec{
				Name:   "extraction",
				Strict: true,
				Schema: responseJSONSchema(),
			},
		},
		Reasoning: reasoningSpec{Enabled: false},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		// Marshaling our own request struct cannot fail in practice; treat
		// it the same as any other attempt failure rather than panicking.
		return attemptResult{failure: FailureReasonHTTPError}
	}

	httpReq, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return attemptResult{failure: FailureReasonHTTPError}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		if attemptCtx.Err() != nil {
			return attemptResult{failure: FailureReasonTimeout, retryable: true}
		}
		return attemptResult{failure: FailureReasonHTTPError, retryable: true}
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if attemptCtx.Err() != nil {
			return attemptResult{failure: FailureReasonTimeout, retryable: true}
		}
		return attemptResult{failure: FailureReasonHTTPError, retryable: true}
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return attemptResult{failure: FailureReasonRateLimited, rawResponse: string(respBody)}
	}
	if resp.StatusCode >= 500 {
		return attemptResult{failure: FailureReasonHTTPError, rawResponse: string(respBody), retryable: true}
	}
	if resp.StatusCode >= 400 {
		return attemptResult{failure: FailureReasonHTTPError, rawResponse: string(respBody)}
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil || len(chatResp.Choices) == 0 {
		return attemptResult{failure: FailureReasonInvalidJSON, rawResponse: string(respBody)}
	}

	content := chatResp.Choices[0].Message.Content

	var raw rawExtraction
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return attemptResult{
			failure:        FailureReasonInvalidJSON,
			rawResponse:    string(respBody),
			promptToks:     chatResp.Usage.PromptTokens,
			completionToks: chatResp.Usage.CompletionTokens,
		}
	}

	if reason := validate(raw, c.ValidZoneIDs); reason != "" {
		return attemptResult{
			failure:        reason,
			rawResponse:    string(respBody),
			promptToks:     chatResp.Usage.PromptTokens,
			completionToks: chatResp.Usage.CompletionTokens,
		}
	}

	return attemptResult{
		raw:            raw,
		rawResponse:    string(respBody),
		promptToks:     chatResp.Usage.PromptTokens,
		completionToks: chatResp.Usage.CompletionTokens,
	}
}

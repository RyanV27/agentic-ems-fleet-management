package agentclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return &Client{
		HTTPClient: http.DefaultClient,
		BaseURL:    server.URL,
		Timeout:    2 * time.Second,
	}
}

func TestTriage_Success(t *testing.T) {
	var gotBody TriageRequest
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/triage", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.WriteHeader(http.StatusOK)
	})

	err := client.Triage(t.Context(), TriageRequest{CallID: "call-1", Reason: "LOW_CONFIDENCE", Attempt: 1})
	require.NoError(t, err)
	assert.Equal(t, "call-1", gotBody.CallID)
	assert.Equal(t, "LOW_CONFIDENCE", gotBody.Reason)
	assert.Equal(t, 1, gotBody.Attempt)
}

func TestTriage_NonSuccessStatus_Errors(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	err := client.Triage(t.Context(), TriageRequest{CallID: "call-1", Reason: "LOW_CONFIDENCE", Attempt: 1})
	require.Error(t, err)
}

func TestTriage_Timeout_Errors(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	client.Timeout = 10 * time.Millisecond

	err := client.Triage(t.Context(), TriageRequest{CallID: "call-1", Reason: "LOW_CONFIDENCE", Attempt: 1})
	require.Error(t, err)
}

func TestTriage_ConnectionRefused_Errors(t *testing.T) {
	client := &Client{
		HTTPClient: http.DefaultClient,
		BaseURL:    "http://127.0.0.1:1",
		Timeout:    2 * time.Second,
	}

	err := client.Triage(t.Context(), TriageRequest{CallID: "call-1", Reason: "LOW_CONFIDENCE", Attempt: 1})
	require.Error(t, err)
}

func TestNewClient_DefaultTimeout(t *testing.T) {
	c := NewClient("http://example.invalid")
	assert.Equal(t, "http://example.invalid", c.BaseURL)
	assert.Greater(t, c.Timeout, time.Duration(0))
	assert.NotNil(t, c.HTTPClient)
}

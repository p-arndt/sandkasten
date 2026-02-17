//go:build integration && linux

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

type testClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newTestClient(baseURL, apiKey string) *testClient {
	return &testClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{},
	}
}

func (c *testClient) doRequest(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.client.Do(req)
	require.NoError(t, err)
	return resp
}

func (c *testClient) createSession(t *testing.T, image string, ttl int) map[string]any {
	t.Helper()
	resp := c.doRequest(t, "POST", "/v1/sessions", map[string]any{
		"image":       image,
		"ttl_seconds": ttl,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create session")
	return decodeResponse(t, resp)
}

func (c *testClient) exec(t *testing.T, sessionID, cmd string) map[string]any {
	t.Helper()
	resp := c.doRequest(t, "POST", fmt.Sprintf("/v1/sessions/%s/exec", sessionID), map[string]any{
		"cmd": cmd,
	})
	return decodeResponse(t, resp)
}

func (c *testClient) writeFile(t *testing.T, sessionID, path, text string) {
	t.Helper()
	resp := c.doRequest(t, "POST", fmt.Sprintf("/v1/sessions/%s/fs/write", sessionID), map[string]any{
		"path": path,
		"text": text,
	})
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func (c *testClient) readFile(t *testing.T, sessionID, path string) map[string]any {
	t.Helper()
	resp := c.doRequest(t, "GET", fmt.Sprintf("/v1/sessions/%s/fs/read?path=%s", sessionID, path), nil)
	return decodeResponse(t, resp)
}

func (c *testClient) destroySession(t *testing.T, sessionID string) {
	t.Helper()
	resp := c.doRequest(t, "DELETE", fmt.Sprintf("/v1/sessions/%s", sessionID), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func decodeResponse(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	defer resp.Body.Close()
	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	return result
}

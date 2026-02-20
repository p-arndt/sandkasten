package protocol

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestRoundtrip(t *testing.T) {
	req := Request{
		ID:        "test-123",
		Type:      RequestExec,
		Cmd:       "echo hello",
		TimeoutMs: 5000,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded Request
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, req.ID, decoded.ID)
	assert.Equal(t, req.Type, decoded.Type)
	assert.Equal(t, req.Cmd, decoded.Cmd)
	assert.Equal(t, req.TimeoutMs, decoded.TimeoutMs)
}

func TestResponseRoundtrip(t *testing.T) {
	resp := Response{
		ID:       "test-456",
		Type:     ResponseExec,
		ExitCode: 0,
		Cwd:      "/workspace",
		Output:   "hello\n",
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded Response
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, resp.ID, decoded.ID)
	assert.Equal(t, resp.Type, decoded.Type)
	assert.Equal(t, resp.ExitCode, decoded.ExitCode)
	assert.Equal(t, resp.Output, decoded.Output)
}

func TestWriteRequestRoundtrip(t *testing.T) {
	req := Request{
		ID:            "w-1",
		Type:          RequestWrite,
		Path:          "/workspace/test.py",
		ContentBase64: "cHJpbnQoImhlbGxvIik=",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded Request
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, RequestWrite, decoded.Type)
	assert.Equal(t, req.Path, decoded.Path)
	assert.Equal(t, req.ContentBase64, decoded.ContentBase64)
	assert.Empty(t, decoded.Text)
}

func TestReadRequestRoundtrip(t *testing.T) {
	req := Request{
		ID:       "r-1",
		Type:     RequestRead,
		Path:     "/workspace/out.txt",
		MaxBytes: 1024,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded Request
	require.NoError(t, json.Unmarshal(data, &decoded))

	assert.Equal(t, RequestRead, decoded.Type)
	assert.Equal(t, req.MaxBytes, decoded.MaxBytes)
}

func TestOmitEmptyFields(t *testing.T) {
	req := Request{
		ID:   "test",
		Type: RequestExec,
		Cmd:  "ls",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	// omitempty fields should not be present
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.NotContains(t, raw, "path")
	assert.NotContains(t, raw, "content_base64")
	assert.NotContains(t, raw, "text")
	assert.NotContains(t, raw, "max_bytes")
}

func TestConstants(t *testing.T) {
	assert.Equal(t, 5*1024*1024, MaxOutputBytes)
	assert.Equal(t, 10*1024*1024, DefaultMaxReadBytes)
	assert.Equal(t, "sandkasten-ws-", WorkspaceVolumePrefix)
	assert.Equal(t, "__SANDKASTEN_BEGIN__", SentinelBegin)
	assert.Equal(t, "__SANDKASTEN_END__", SentinelEnd)
}

func TestRequestTypes(t *testing.T) {
	assert.Equal(t, RequestType("exec"), RequestExec)
	assert.Equal(t, RequestType("exec_stream"), RequestExecStream)
	assert.Equal(t, RequestType("write"), RequestWrite)
	assert.Equal(t, RequestType("read"), RequestRead)
}

func TestResponseTypes(t *testing.T) {
	assert.Equal(t, ResponseType("exec"), ResponseExec)
	assert.Equal(t, ResponseType("exec_chunk"), ResponseExecChunk)
	assert.Equal(t, ResponseType("exec_done"), ResponseExecDone)
	assert.Equal(t, ResponseType("write"), ResponseWrite)
	assert.Equal(t, ResponseType("read"), ResponseRead)
	assert.Equal(t, ResponseType("error"), ResponseError)
	assert.Equal(t, ResponseType("ready"), ResponseReady)
}

func TestReadyMessage(t *testing.T) {
	msg := ReadyMessage{Type: ResponseReady}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded ReadyMessage
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, ResponseReady, decoded.Type)
}

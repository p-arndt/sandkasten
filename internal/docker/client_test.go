package docker

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindJSONLine_CleanJSON(t *testing.T) {
	input := []byte(`{"id":"test","type":"exec","exit_code":0}`)
	result := findJSONLine(input)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), `"id":"test"`)
}

func TestFindJSONLine_DockerHeaders(t *testing.T) {
	// Docker multiplexed stream has header bytes before JSON
	input := []byte("\x01\x00\x00\x00\x00\x00\x00\x2a{\"id\":\"test\",\"type\":\"exec\"}")
	result := findJSONLine(input)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), `"id":"test"`)
}

func TestFindJSONLine_NoJSON(t *testing.T) {
	input := []byte("no json here at all")
	result := findJSONLine(input)
	assert.Nil(t, result)
}

func TestFindJSONLine_MultipleLines(t *testing.T) {
	input := []byte("some log output\n{\"id\":\"test\",\"type\":\"exec\"}\nmore output\n")
	result := findJSONLine(input)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), `"id":"test"`)
}

func TestFindJSONLine_EmptyInput(t *testing.T) {
	result := findJSONLine([]byte{})
	assert.Nil(t, result)
}

func TestFindJSONLine_BinaryPrefix(t *testing.T) {
	// Simulates Docker's stream multiplexing header bytes
	input := append([]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x30},
		[]byte(`{"type":"ready"}`)...)
	result := findJSONLine(input)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), `"type":"ready"`)
}

package herrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorJSON(t *testing.T) {
	assert := assert.New(t)
	herr := New("ERR-123", "An error", map[string]interface{}{
		"some": "details",
	}, errors.New("error"))

	herrJSON, err := json.Marshal(herr)
	assert.NoError(err)

	assert.Equal(`{"error":"ERR-123","message":"An error","details":{"some":"details"}}`, string(herrJSON))
}

func TestPlatformOrchestratorError_WriteToHttpWithErrLogger(t *testing.T) {
	rec := httptest.NewRecorder()
	herr := NewWithStatus(400, "not-found", errors.New("foo"))
	messages := make([]string, 0)
	herr.WriteToHttpWithErrLogger(rec, func(s string, i ...interface{}) {
		messages = append(messages, fmt.Sprintf("%s %v", s, i))
	})
	assert.Equal(t, 400, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("content-type"))
	assert.Equal(t, "{\"error\":\"HTTP-400\",\"message\":\"not-found\"}\n", string(rec.Body.Bytes()))
	assert.Equal(t, []string{"writing error response with go error [err foo]"}, messages)
}

func TestNewInternalError(t *testing.T) {
	herr := NewInternalError(errors.New("foo"))
	raw, err := json.Marshal(herr)
	assert.NoError(t, err)
	assert.Equal(t, "{\"error\":\"HTTP-500\",\"message\":\"Unexpected error\"}", string(raw))
}

func TestPlatformOrchestratorError_WithConventions(t *testing.T) {
	herr := new(PlatformOrchestratorError).WithConventions()
	raw, err := json.Marshal(herr)
	assert.NoError(t, err)
	assert.Equal(t, "{\"error\":\"HTTP-0\",\"message\":\"Unexpected error\"}", string(raw))

	herr.Code = "HTTP-999"
	raw, err = json.Marshal(herr)
	assert.NoError(t, err)
	assert.Equal(t, "{\"error\":\"HTTP-999\",\"message\":\"Unexpected error\"}", string(raw))
}

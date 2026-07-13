package hservicejwt

import (
	"net/textproto"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHttpHeader(t *testing.T) {
	assert.Equal(t, "X-Service-Authorization", HttpHeader)
	assert.Equal(t, HttpHeader, textproto.CanonicalMIMEHeaderKey(HttpHeader))
}

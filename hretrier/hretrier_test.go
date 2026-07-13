package hretrier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/justinrixx/retryhttp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeRt func(request *http.Request) (*http.Response, error)

func (f fakeRt) RoundTrip(request *http.Request) (*http.Response, error) {
	return (func(request *http.Request) (*http.Response, error))(f)(request)
}

var _ http.RoundTripper = (*fakeRt)(nil)

func TestWrapHttpClientWithStandardRetries_none(t *testing.T) {
	called := 0
	c := &http.Client{
		Transport: fakeRt(func(request *http.Request) (*http.Response, error) {
			called += 1
			return nil, fmt.Errorf("no retry")
		}),
	}
	c = WrapHttpClientWithStandardRetries(c)
	_, err := c.Get("http://localhost/example")
	require.EqualError(t, err, "Get \"http://localhost/example\": no retry")
	assert.Equal(t, 1, called)
}

func TestWrapHttpClientWithStandardRetries_no_retry_post(t *testing.T) {
	called := 0
	c := &http.Client{
		Transport: fakeRt(func(request *http.Request) (*http.Response, error) {
			called += 1
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader([]byte{}))}, nil
		}),
	}
	c = WrapHttpClientWithStandardRetries(c)
	req, _ := http.NewRequest(http.MethodPost, "http://localhost/example", nil)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, 1, called)
}

func TestWrapHttpClientWithStandardRetries_idempotent_500(t *testing.T) {
	called := 0
	c := &http.Client{
		Transport: fakeRt(func(request *http.Request) (*http.Response, error) {
			called += 1
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader([]byte{}))}, nil
		}),
	}
	c = WrapHttpClientWithStandardRetries(c)
	res, err := c.Get("http://localhost/example")
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, 3, called)
}

func TestStandardRetryAnyMethod_POST(t *testing.T) {
	called := 0
	c := &http.Client{
		Transport: fakeRt(func(request *http.Request) (*http.Response, error) {
			called += 1
			return &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(bytes.NewReader([]byte{}))}, nil
		}),
	}
	c = WrapHttpClientWithStandardRetries(c)
	req, _ := http.NewRequestWithContext(retryhttp.SetShouldRetryFn(context.Background(), StandardRetryAnyMethod), http.MethodPost, "http://localhost/example", nil)
	res, err := c.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	assert.Equal(t, 3, called)
}

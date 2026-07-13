// Package hretrier provides a standard HTTP retry client configuration for our service-to-service calls. This will
// perform 2 retries for some general internal failure types. See WrapHttpClientWithStandardRetries.
package hretrier

import (
	"net/http"

	"github.com/justinrixx/retryhttp"
)

// BaseStandardRetryFn is the base standard retry function used by default in WrapHttpClientWithStandardRetries.
var BaseStandardRetryFn = retryhttp.CustomizedShouldRetryFn(retryhttp.CustomizedShouldRetryFnOptions{
	// The general idempotent methods that we use in our service-to-service calls
	IdempotentMethods: []string{http.MethodGet, http.MethodDelete, http.MethodHead, http.MethodPut},
	RetryableStatusCodes: []int{
		// We're ok with retrying 500's on internal server errors for these idempotent methods
		http.StatusInternalServerError,
		// General bad gateway and downstream errors can also be retried
		http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout,
	},
})

// StandardRetryAnyMethod wraps BaseStandardRetryFn but treats all http methods as retryable, including POST, PATCH,
// etc.
var StandardRetryAnyMethod = func(attempt retryhttp.Attempt) bool {
	// For the sake of calculating the retry - treat this request as idempotent
	beforeMethod := attempt.Req.Method
	defer func() {
		attempt.Req.Method = beforeMethod
	}()
	attempt.Req.Method = http.MethodGet
	return BaseStandardRetryFn(attempt)
}

// WrapHttpClientWithStandardRetries wraps the client with default HTTP retries as used by platform-orchestrator service-to-service
// calls. This modifies the passed-in client and returns it.
func WrapHttpClientWithStandardRetries(c *http.Client) *http.Client {
	c.Transport = retryhttp.New(
		// Inject the previous clients transport in case that has customised behavior already that we need to wrap.
		// For example, datadog tracing.
		retryhttp.WithTransport(c.Transport),
		// Add 2 retries by default
		retryhttp.WithMaxRetries(2),
		// Default retry delay function
		retryhttp.WithDelayFn(retryhttp.DefaultDelayFn),
		// apply our standard retry function
		retryhttp.WithShouldRetryFn(BaseStandardRetryFn),
	)
	return c
}

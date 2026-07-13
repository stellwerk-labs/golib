package httplogger

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/stellwerk-labs/golib/hlogger"
)

var (
	testResponse     = "resp"
	testResponseSize = int64(len(testResponse))
	testPath         = "/alive"
)

func testServer(config *Config, statusCode int) *httptest.Server {
	router := mux.NewRouter()
	router.Methods("GET").Path(testPath).HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ids, ok := hlogger.GetPlatformOrchestratorIdsFromCtx(r.Context()); ok {
			ids.OrgId = "example-org"
		}
		w.WriteHeader(statusCode)
		w.Write([]byte(testResponse))
	})
	router.Use(LoggingMiddleware(config))

	return httptest.NewServer(router)
}

func testRequest(assert *assert.Assertions, srv *httptest.Server, expectedStatusCode int) {
	res, err := http.Get(srv.URL + testPath)
	assert.NoError(err)
	assert.Equal(expectedStatusCode, res.StatusCode)

	defer res.Body.Close()
	out, err := ioutil.ReadAll(res.Body)
	assert.NoError(err)
	assert.Equal("resp", string(out))
}

func TestLoggingMiddleware(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	config := &Config{
		Logger: zap.New(observer),
	}

	srv := testServer(config, http.StatusOK)
	defer srv.Close()

	testRequest(assert, srv, http.StatusOK)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal(logCtx["http"], map[string]interface{}{
		"method":        "GET",
		"response_size": testResponseSize,
		"status_code":   200,
		"url":           testPath,
		"useragent":     "Go-http-client/1.1",
		"version":       "HTTP/1.1",
	})
	assert.Greater(logCtx["duration"], int64(1))
}

func TestLoggingMiddlewareSilencedPathSucceeding(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	config := &Config{
		Logger:        zap.New(observer),
		SilencedPaths: []string{testPath},
	}

	srv := testServer(config, http.StatusOK)
	defer srv.Close()

	testRequest(assert, srv, http.StatusOK)

	logged := logs.All()
	assert.Len(logged, 0)
}

func TestLoggingMiddlewareSilencedPathSucceedingAndDebug(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.DebugLevel)
	config := &Config{
		Logger:        zap.New(observer),
		SilencedPaths: []string{testPath},
	}

	srv := testServer(config, http.StatusOK)
	defer srv.Close()

	testRequest(assert, srv, http.StatusOK)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled silenced path", log.Message)
	logCtx := log.ContextMap()
	assert.Equal("example-org", logCtx["po-org-id"])
	assert.Equal(logCtx["http"], map[string]interface{}{
		"method":        "GET",
		"response_size": testResponseSize,
		"status_code":   200,
		"url":           testPath,
		"useragent":     "Go-http-client/1.1",
		"version":       "HTTP/1.1",
	})
	assert.Greater(logCtx["duration"], int64(1))
}

func TestLoggingMiddlewareSilencedPathFailing(t *testing.T) {
	assert := assert.New(t)
	observer, logs := observer.New(zap.InfoLevel)
	config := &Config{
		Logger:        zap.New(observer),
		SilencedPaths: []string{testPath},
	}

	srv := testServer(config, http.StatusInternalServerError)
	defer srv.Close()

	testRequest(assert, srv, http.StatusInternalServerError)

	logged := logs.All()
	assert.Len(logged, 1)

	log := logged[0]
	assert.Equal("handled", log.Message)
	logCtx := log.ContextMap()
	assert.Equal("example-org", logCtx["po-org-id"])
	assert.Equal(logCtx["http"], map[string]interface{}{
		"method":        "GET",
		"response_size": testResponseSize,
		"status_code":   500,
		"url":           testPath,
		"useragent":     "Go-http-client/1.1",
		"version":       "HTTP/1.1",
	})
	assert.Greater(logCtx["duration"], int64(1))
}

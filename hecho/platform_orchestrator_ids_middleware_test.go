package hecho

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"

	"github.com/stellwerk-labs/golib/hlogger"
)

func TestPlatformOrchestratorIdsMiddleware(t *testing.T) {
	es := echo.New()
	es.Use(PlatformOrchestratorIdsMiddleware)

	var innerCtx context.Context
	es.Router().Add(http.MethodGet, "/o/:orgId/e/:envId/d/:deployId", func(c echo.Context) error {
		innerCtx = c.Request().Context()
		return nil
	})

	t.Run("with match - no parent ctx", func(t *testing.T) {
		innerCtx = nil
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/o/my-org/e/my-env/d/my-deploy", nil)
		es.ServeHTTP(resp, req)
		if assert.NotNil(t, innerCtx) {
			ids, ok := hlogger.GetPlatformOrchestratorIdsFromCtx(innerCtx)
			assert.True(t, ok)
			assert.Equal(t, hlogger.StandardPlatformOrchestratorIDs{
				OrgId: "my-org", EnvId: "my-env", DeployId: "my-deploy",
			}, *ids)
		}
	})

	t.Run("with match", func(t *testing.T) {
		innerCtx = nil
		resp := httptest.NewRecorder()
		ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/o/my-org/e/my-env/d/my-deploy", nil).WithContext(ctx)
		es.ServeHTTP(resp, req)
		assert.Equal(t, hlogger.StandardPlatformOrchestratorIDs{
			OrgId: "my-org", EnvId: "my-env", DeployId: "my-deploy",
		}, *ids)
	})

	t.Run("no match", func(t *testing.T) {
		innerCtx = nil
		resp := httptest.NewRecorder()
		ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(context.Background())
		req := httptest.NewRequest(http.MethodGet, "/bananas", nil).WithContext(ctx)
		es.ServeHTTP(resp, req)
		assert.Nil(t, innerCtx)
		assert.Equal(t, hlogger.StandardPlatformOrchestratorIDs{}, *ids)
	})
}

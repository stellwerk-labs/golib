package hlogger

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

func TestGetPlatformOrchestratorIdsFromCtx(t *testing.T) {
	ctx := context.Background()
	ids, ok := GetPlatformOrchestratorIdsFromCtx(ctx)
	assert.Nil(t, ids)
	assert.False(t, ok)
	ids, ctx = EnsurePlatformOrchestratorIdsOnCtx(ctx)
	assert.NotNil(t, ids)
	ids, ok = GetPlatformOrchestratorIdsFromCtx(ctx)
	assert.NotNil(t, ids)
	assert.True(t, ok)

	ctx2 := context.WithValue(ctx, "a", "b")
	ids2, ok := GetPlatformOrchestratorIdsFromCtx(ctx2)
	assert.NotNil(t, ids2)
	assert.True(t, ok)
	assert.Same(t, ids, ids2)

	ids2.OrgId = "hello"
	assert.Equal(t, "hello", ids.OrgId)
}

func TestEnsurePlatformOrchestratorIdsOnCtx(t *testing.T) {
	ctx := context.Background()
	ids, ctx := EnsurePlatformOrchestratorIdsOnCtx(ctx)
	assert.NotNil(t, ids)
	ids2, ctx2 := EnsurePlatformOrchestratorIdsOnCtx(ctx)
	assert.NotNil(t, ids)
	assert.Same(t, ids, ids2)
	assert.Same(t, ctx, ctx2)
}

func TestAsLogFields(t *testing.T) {
	ids := new(StandardPlatformOrchestratorIDs)
	assert.Empty(t, ids.AsLogFields())

	ids.OrgId = "a"
	ids.AppId = "b"
	ids.EnvId = "c"
	ids.DeployId = "d"
	ids.PipelineId = "e"
	ids.PipelineRunId = "f"
	ids.AgentSessionId = "g"
	ids.AgentFingerPrint = "h"
	ids.AgentId = "i"
	ids.ResourceGuId = "j"
	ids.ResourceDefId = "k"

	fields := ids.AsLogFields()
	assert.Len(t, fields, 11)

	oe := zapcore.NewMapObjectEncoder()
	ids.AsLogField().AddTo(oe)
	assert.Equal(t, map[string]interface{}{
		"po-org-id":            "a",
		"po-app-id":            "b",
		"po-env-id":            "c",
		"po-deploy-id":         "d",
		"po-pip-id":            "e",
		"po-run-id":            "f",
		"po-agent-session-id":  "g",
		"po-agent-fingerprint": "h",
		"po-agent-id":          "i",
		"po-gu-res-id":         "j",
		"po-res-def-id":        "k",
	}, oe.Fields)
}

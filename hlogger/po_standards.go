package hlogger

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/stellwerk-labs/golib/htelemetry"
)

const (
	POOrgId            = "po-org-id"
	POUserId           = "po-user-id"
	POAppId            = "po-app-id"
	POProjectId        = "po-project-id"
	POEnvId            = "po-env-id"
	PODeployId         = "po-deploy-id"
	PORunnerId         = "po-runner-id"
	POPipelineId       = "po-pip-id"
	POPipelineRunId    = "po-run-id"
	POAgentId          = "po-agent-id"
	POAgentSessionId   = "po-agent-session-id"
	POAgentFingerPrint = "po-agent-fingerprint"
	POResourceGuId     = "po-gu-res-id"
	POResourceDefId    = "po-res-def-id"
	POResourceAccId    = "po-acc-id"
)

// StandardPlatformOrchestratorIDs holds a set of common IDs associated with the current context. These can be added to a zap
// logger when necessary using AsLogFields.
type StandardPlatformOrchestratorIDs struct {
	UserId           string
	OrgId            string
	AppId            string
	ProjectId        string
	EnvId            string
	DeployId         string
	RunnerId         string
	PipelineId       string
	PipelineRunId    string
	AgentId          string
	AgentSessionId   string
	AgentFingerPrint string
	ResourceGuId     string
	ResourceDefId    string
	ResourceAccId    string
}

// AsLogFields converts all of the non-empty fields into appropriate id's on the logger as strongly typed
// string fields.
// Deprecated. Please use AsLogField instead.
func (ids *StandardPlatformOrchestratorIDs) AsLogFields() []zap.Field {
	output := make([]zap.Field, 0, 6)
	ids.Iter()(func(k, v string) bool {
		output = append(output, zap.String(k, v))
		return true
	})
	return output
}

// AsLogField adds the ids collection to the logger as a lazy inline field which will get marshalled later.
func (ids *StandardPlatformOrchestratorIDs) AsLogField() zap.Field {
	return zap.Inline(ids)
}

var _ zapcore.ObjectMarshaler = (*StandardPlatformOrchestratorIDs)(nil)

func (ids *StandardPlatformOrchestratorIDs) Iter() func(func(k, v string) bool) {
	return func(yield func(k string, v string) bool) {
		if ids.UserId != "" && !yield(POUserId, ids.UserId) {
			return
		} else if ids.OrgId != "" && !yield(POOrgId, ids.OrgId) {
			return
		} else if ids.AppId != "" && !yield(POAppId, ids.AppId) {
			return
		} else if ids.ProjectId != "" && !yield(POProjectId, ids.ProjectId) {
			return
		} else if ids.EnvId != "" && !yield(POEnvId, ids.EnvId) {
			return
		} else if ids.DeployId != "" && !yield(PODeployId, ids.DeployId) {
			return
		} else if ids.RunnerId != "" && !yield(PORunnerId, ids.RunnerId) {
			return
		} else if ids.PipelineId != "" && !yield(POPipelineId, ids.PipelineId) {
			return
		} else if ids.PipelineRunId != "" && !yield(POPipelineRunId, ids.PipelineRunId) {
			return
		} else if ids.AgentId != "" && !yield(POAgentId, ids.AgentId) {
			return
		} else if ids.AgentSessionId != "" && !yield(POAgentSessionId, ids.AgentSessionId) {
			return
		} else if ids.AgentFingerPrint != "" && !yield(POAgentFingerPrint, ids.AgentFingerPrint) {
			return
		} else if ids.ResourceGuId != "" && !yield(POResourceGuId, ids.ResourceGuId) {
			return
		} else if ids.ResourceDefId != "" && !yield(POResourceDefId, ids.ResourceDefId) {
			return
		} else if ids.ResourceAccId != "" && !yield(POResourceAccId, ids.ResourceAccId) {
			return
		}
	}
}

func (ids *StandardPlatformOrchestratorIDs) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	ids.Iter()(func(k, v string) bool {
		enc.AddString(k, v)
		return true
	})
	return nil
}

// SetOnTracingSpan sets the platform-orchestrator IDs as tags on a Datadog span.
// Deprecated: Use SetOnSpan instead, which works with both DD and OTel providers.
func (ids *StandardPlatformOrchestratorIDs) SetOnTracingSpan(span ddtrace.Span) {
	ids.Iter()(func(k, v string) bool {
		span.SetTag(k, v)
		return true
	})
}

// SetOnTracingCtxSpan sets the platform-orchestrator IDs on the Datadog span in context.
// Deprecated: Use SetOnCtxSpan instead, which works with both DD and OTel providers.
func (ids *StandardPlatformOrchestratorIDs) SetOnTracingCtxSpan(ctx context.Context) {
	if span, ok := tracer.SpanFromContext(ctx); ok {
		ids.SetOnTracingSpan(span)
	}
}

// SetOnSpan sets the platform-orchestrator IDs as tags on an htelemetry.Span.
// This is a provider-agnostic alternative to SetOnTracingSpan that works with both
// Datadog and OpenTelemetry backends.
func (ids *StandardPlatformOrchestratorIDs) SetOnSpan(span htelemetry.Span) {
	ids.Iter()(func(k, v string) bool {
		span.SetTag(k, v)
		return true
	})
}

// SetOnCtxSpan sets the platform-orchestrator IDs on the span in context using the global telemetry provider.
// This is a provider-agnostic alternative to SetOnTracingCtxSpan that works with both
// Datadog and OpenTelemetry backends.
func (ids *StandardPlatformOrchestratorIDs) SetOnCtxSpan(ctx context.Context) {
	if span, ok := htelemetry.SpanFromContext(ctx); ok {
		ids.SetOnSpan(span)
	}
}

type contextKeyType string

const platformOrchestratorIDsKey = contextKeyType("standard-platform-orchestrator-ids")

// GetPlatformOrchestratorIdsFromCtx will check the current context for the standard platform-orchestrator ids and return them if they exist
// or return nil if they don't exist. An additional boolean is returned to make this more safe from NPEs. Note that
// this returns a pointer to the object on the context, and all child contexts will inherit this pointer. So
// modifications to this object will propagate up and down the stack.
func GetPlatformOrchestratorIdsFromCtx(ctx context.Context) (*StandardPlatformOrchestratorIDs, bool) {
	v, ok := ctx.Value(platformOrchestratorIDsKey).(*StandardPlatformOrchestratorIDs)
	if !ok {
		return nil, false
	}
	return v, true
}

// EnsurePlatformOrchestratorIdsOnCtx is like GetPlatformOrchestratorIdsFromCtx but will additionally set the field to an empty struct if
// it hasn't been called already.
func EnsurePlatformOrchestratorIdsOnCtx(ctx context.Context) (*StandardPlatformOrchestratorIDs, context.Context) {
	if ids, ok := GetPlatformOrchestratorIdsFromCtx(ctx); ok {
		return ids, ctx
	}
	return ResetPlatformOrchestratorIdsOnCtx(ctx)
}

// ResetPlatformOrchestratorIdsOnCtx will reset the structure on this context without affecting the parent or any existing child
// contexts.
func ResetPlatformOrchestratorIdsOnCtx(ctx context.Context) (*StandardPlatformOrchestratorIDs, context.Context) {
	ids := new(StandardPlatformOrchestratorIDs)
	return ids, context.WithValue(ctx, platformOrchestratorIDsKey, ids)
}

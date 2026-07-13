package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/wagslane/go-rabbitmq"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/stellwerk-labs/golib/hlogger"
	"github.com/stellwerk-labs/golib/hrabbitmq"
	"github.com/stellwerk-labs/golib/htelemetry"
)

func WrapWithTimeout(timeout time.Duration, next HandlerFunc) HandlerFunc {
	return func(ctx context.Context, logger *zap.Logger, delivery *rabbitmq.Delivery) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return next(ctx, logger, delivery)
	}
}

// WrapLoggerAndTracer extracts fields from the delivery and adds them to the platform-orchestrator ids context.
// It also catches and logs any panic or error from the handler.
// This function is provider-agnostic and works with both Datadog and OpenTelemetry backends.
func WrapLoggerAndTracer(operationName string, next HandlerFunc) HandlerFunc {
	withPanicRecovery := func(ctx context.Context, logger *zap.Logger, delivery *rabbitmq.Delivery) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("caught panic: %v", r)
			}
		}()
		err = next(ctx, logger, delivery)
		return
	}
	return func(ctx context.Context, logger *zap.Logger, d *rabbitmq.Delivery) (err error) {

		// Normalize the routing key for the org-scoped messages. This improves our observability so that similar
		// messages are grouped together without the org-name.
		if strings.HasPrefix(d.RoutingKey, "organization.") {
			parts := strings.SplitN(d.RoutingKey, ".", 3)
			parts[1] = "*"
			d.RoutingKey = strings.Join(parts, ".")
		}

		// attempt to get the parent span ids from the message using provider-agnostic extraction
		startSpanOptions := hrabbitmq.ExtractSpanOptionsFromMessage(logger, d.Headers)
		startSpanOptions = append(startSpanOptions, htelemetry.ResourceName(d.RoutingKey))

		// create a new span and embed it onto the context using the global provider
		span := htelemetry.StartSpan(operationName, startSpanOptions...)
		ctx = htelemetry.ContextWithSpan(ctx, span)

		span.SetTag("rabbitmq.routing-key", d.RoutingKey)
		// if the message is not present, lets assign one so we can better track this message
		if d.MessageId == "" {
			d.MessageId = strconv.Itoa(int(rand.Int64()))
		}
		span.SetTag("rabbitmq.message-id", d.MessageId)
		// capture the _original_ routing key here before it gets modified by the handlers
		originalRk := d.RoutingKey

		retryAttempt, _ := d.Headers[RetryAttemptTrackerHeader].(int32)
		// create a new logger for the span embedded on the context using provider-agnostic function
		logger = hlogger.TraceScopedLoggerFromSpan(logger, span).With(
			zap.Object("rabbitmq", zapcore.ObjectMarshalerFunc(func(encoder zapcore.ObjectEncoder) error {
				encoder.AddString("routing-key", originalRk)
				encoder.AddString("message-id", d.MessageId)
				if retryAttempt > 0 {
					encoder.AddInt32("retry-attempt", retryAttempt)
				}
				return nil
			})),
		)

		// add platform-orchestrator ids to the logger and span
		ids, ctx := hlogger.EnsurePlatformOrchestratorIdsOnCtx(ctx)
		extractPlatformOrchestratorIdsFromDelivery(logger, ids, d)
		logger = logger.WithLazy(ids.AsLogField())
		err = withPanicRecovery(ctx, logger, d)

		if err != nil {
			logger.With(zap.Error(err)).Error("Failed while handling message")
			span.Finish(htelemetry.WithError(err))
			return err
		}
		logger.Debug("Handled message successfully")
		span.Finish()
		return nil
	}
}

func extractPlatformOrchestratorIdsFromDelivery(logger *zap.Logger, ids *hlogger.StandardPlatformOrchestratorIDs, d *rabbitmq.Delivery) {
	if d.ContentType != "application/json" || d.Body == nil {
		logger.Debug("failed to decode rabbitmq message as json: bad content type or no body", zap.String("ct", d.ContentType), zap.Int("#body", len(d.Body)))
		return
	}

	type anon struct {
		Data struct {
			OrgId                string `json:"organization_id"`
			AppId                string `json:"application_id"`
			EnvId                string `json:"environment_id"`
			DeployId             string `json:"deploy_id"`
			ResourceDefinitionId string `json:"resource_definition_id,omitempty"`
		} `json:"data"`
	}

	var ew anon
	if err := json.Unmarshal(d.Body, &ew); err == nil {
		ids.OrgId = ew.Data.OrgId
		ids.AppId = ew.Data.AppId
		ids.EnvId = ew.Data.EnvId
		ids.DeployId = ew.Data.DeployId
		ids.ResourceDefId = ew.Data.ResourceDefinitionId
	}
}

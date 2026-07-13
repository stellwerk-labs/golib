package hecho

import (
	"context"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	oaiechomiddleware "github.com/oapi-codegen/echo-middleware"
)

func DefaultOAIValidationSkipper(c echo.Context) bool {
	return c.Path() == "/alive" || c.Path() == "/health" || c.Path() == "/docs/spec.json" || c.Path() == "/docs/spec.yaml"
}

// oapiValidatorFromYaml is an Echo middleware function which validates incoming HTTP requests
// to make sure that they conform to the given OAPI 3.0 specification. When
// OAPI validation fails on the request, we return an HTTP/400.
// Create validator middleware from the content of a YAML document.
func oapiValidatorFromYaml(yaml []byte, skipperFn echomiddleware.Skipper) (echo.MiddlewareFunc, error) {
	swagger, err := openapi3.NewLoader().LoadFromData(yaml)
	if err != nil {
		return nil, fmt.Errorf("error parsing data as Swagger YAML: %s", err)
	}

	filterOptions := &openapi3filter.Options{
		AuthenticationFunc: func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
			return nil
		},
	}
	// The json schema errors that get emitted can sometimes be very hostile to the user when the pattern is complex.
	// This error customizer allows an x-pattern-error to be defined in the field schema, alongside the pattern, and
	// this will be returned instead if the field is not valid.
	filterOptions.WithCustomSchemaErrorFunc(func(err *openapi3.SchemaError) string {
		if err.SchemaField == "pattern" {
			if pv, ok := err.Schema.Extensions["x-pattern-error"].(string); ok && len(pv) > 0 {
				return pv
			}
		}
		return ""
	})
	validatorOptions := &oaiechomiddleware.Options{
		Skipper: DefaultOAIValidationSkipper,
		Options: *filterOptions,
	}

	if skipperFn != nil {
		validatorOptions.Skipper = skipperFn
	}

	return oaiechomiddleware.OapiRequestValidatorWithOptions(swagger, validatorOptions), nil
}

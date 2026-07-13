package henvflags

import (
	"context"
	"testing"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/stretchr/testify/assert"
)

func TestNominal(t *testing.T) {

	p := New(&OverrideVariant{
		Flag: "example-string", OverrideValue: "string", MatchCriteria: map[string]string{"x": "y"},
	})
	assert.NoError(t, p.AddJsonOverrides(`[
	{"flag": "example-int", "value": 42, "criteria": {"x": "y"}},
	{"flag": "example-flag", "value": true, "criteria": {"x": "re:abcd.*"}},
    {"flag": "example-obj", "value": ["a", "b"], "criteria": {"x": "z"}}
]`))
	assert.NoError(t, openfeature.SetProviderAndWait(p))
	defer func() {
		assert.NoError(t, openfeature.SetProviderAndWait(openfeature.NoopProvider{}))
	}()
	c := openfeature.NewClient("foo")

	v, err := c.BooleanValue(context.Background(), "something", true, openfeature.NewEvaluationContext("eg", map[string]interface{}{}))
	assert.NoError(t, err)
	assert.True(t, v)

	var vStr string
	vStr, err = c.StringValue(context.Background(), "example-string", "", openfeature.NewEvaluationContext("eg", map[string]interface{}{}))
	assert.NoError(t, err)
	assert.Equal(t, "", vStr)
	vStr, err = c.StringValue(context.Background(), "example-string", "", openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "z"}))
	assert.NoError(t, err)
	assert.Equal(t, "", vStr)
	vStr, err = c.StringValue(context.Background(), "example-string", "", openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "y"}))
	assert.NoError(t, err)
	assert.Equal(t, "string", vStr)
	vStrDetails, err := c.StringValueDetails(context.Background(), "example-string", "", openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "y"}))
	assert.NoError(t, err)
	assert.Equal(t, "string", vStrDetails.Value)
	assert.Equal(t, openfeature.TargetingMatchReason, vStrDetails.Reason)
	vStrDetails, err = c.StringValueDetails(context.Background(), "example-string", "", openfeature.NewEvaluationContext("eg", map[string]interface{}{}))
	assert.NoError(t, err)
	assert.Equal(t, "", vStrDetails.Value)
	assert.Equal(t, openfeature.DefaultReason, vStrDetails.Reason)

	var vInt int64
	vInt, err = c.IntValue(context.Background(), "example-int", int64(-1), openfeature.NewEvaluationContext("eg", map[string]interface{}{}))
	assert.NoError(t, err)
	assert.Equal(t, int64(-1), vInt)
	vInt, err = c.IntValue(context.Background(), "example-int", -1, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "y"}))
	assert.NoError(t, err)
	assert.Equal(t, int64(42), vInt)

	vBool, err := c.BooleanValue(context.Background(), "example-flag", false, openfeature.NewEvaluationContext("eg", map[string]interface{}{}))
	assert.NoError(t, err)
	assert.Equal(t, false, vBool)
	vBool, err = c.BooleanValue(context.Background(), "example-flag", false, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "a"}))
	assert.NoError(t, err)
	assert.Equal(t, false, vBool)
	vBool, err = c.BooleanValue(context.Background(), "example-flag", false, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "abcdef"}))
	assert.NoError(t, err)
	assert.Equal(t, true, vBool)
	vBool, err = c.BooleanValue(context.Background(), "example-flag", false, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "aabcde"}))
	assert.NoError(t, err)
	assert.Equal(t, false, vBool)

	vObj, err := c.ObjectValue(context.Background(), "example-obj", []interface{}{}, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "z"}))
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{"a", "b"}, vObj)
	vObj, err = c.ObjectValue(context.Background(), "example-obj", []interface{}{}, openfeature.NewEvaluationContext("eg", map[string]interface{}{"x": "not-z"}))
	assert.NoError(t, err)
	assert.Equal(t, []interface{}{}, vObj)
}

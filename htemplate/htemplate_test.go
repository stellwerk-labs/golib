package htemplate

import (
	"bytes"
	"testing"
	ttemplate "text/template"

	"github.com/stretchr/testify/assert"
)

func TestTxtFuncMap(t *testing.T) {
	assert := assert.New(t)
	f := TxtFuncMap()

	template := `Hello {{.Name | trim | lower}}`
	data := map[string]interface{}{"Name": "  John Jacob Jingleheimer Schmidt "}

	goTmpl, err := ttemplate.New("my-template").Funcs(f).Parse(template)
	assert.NoError(err)

	var yamlBuffer bytes.Buffer
	err = goTmpl.Execute(&yamlBuffer, data)
	assert.NoError(err)

	expectedEvaluationResult := "Hello john jacob jingleheimer schmidt"
	assert.Equal(expectedEvaluationResult, yamlBuffer.String())
}

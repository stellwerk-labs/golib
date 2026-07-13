# htemplate

Shared module that exports a collection of [sprig](https://masterminds.github.io/sprig/) functions that may be used to evaluate go templates and are supposed to be safe to use.

## using htemplate

Using `amqp091`

```golang
f := htemplate.TxtFuncMap()
template := `Hello {{.Name | trim | lower}}`
data := map[string]interface{}{"Name": "  John Jacob Jingleheimer Schmidt "}

goTmpl, err := ttemplate.New("my-template").Funcs(f).Parse(template)
if err != nil {
  return err
}


var yamlBuffer bytes.Buffer
err = goTmpl.Execute(&yamlBuffer, data)
if err != nil {
  return err
}
```

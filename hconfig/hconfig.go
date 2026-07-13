package hconfig

import (
	"context"
	"fmt"
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/jinzhu/copier"
	"github.com/sethvargo/go-envconfig"
	"github.com/sevlyar/retag"
)

func LoadConfig[E any](conf *E) error {
	patchedConf := replaceLegacyMapstructureTags(conf)

	if err := LoadConfigWithoutRetag(patchedConf); err != nil {
		return err
	}

	copier.Copy(&conf, &patchedConf)

	return nil
}

func LoadConfigWithoutRetag[E any](conf E) error {
	ctx := context.Background()

	if err := envconfig.ProcessWith(ctx, &envconfig.Config{
		DefaultOverwrite: true,
		DefaultNoInit:    true,
		Target:           conf,
	}); err != nil {
		return err
	}

	validate := validator.New()
	if err := validate.Struct(conf); err != nil {
		return err
	}

	return nil
}

type mapstructureConverter struct{}

func (m mapstructureConverter) MakeTag(t reflect.Type, fieldIndex int) reflect.StructTag {
	field := t.Field(fieldIndex)
	value := field.Tag.Get("mapstructure")
	if value != "" {
		return field.Tag + reflect.StructTag(fmt.Sprintf(`env:"%s"`, value))
	}
	return field.Tag
}

// replaceLegacyMapstructureTags replaces "mapstructure" tags with "env" tags
func replaceLegacyMapstructureTags[E any](conf *E) interface{} {
	return retag.Convert(conf, mapstructureConverter{})
}

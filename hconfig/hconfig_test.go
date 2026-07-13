package hconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

type jsonMap map[string]interface{}

func (m *jsonMap) EnvDecode(val string) error {
	return json.Unmarshal([]byte(val), m)
}

// Configuration ...
type configuration struct {
	DataDogEnabled bool     `env:"DD_ENABLE"`
	LogLevel       string   `env:"LOG_LEVEL"`
	Port           int      `env:"PORT"`
	Host           string   `env:"HOST, default=my-host"`
	DatabasePort   int      `env:"DATABASE_PORT" validate:"required"`
	Params         *jsonMap `env:"PARAMS"`
}

// Configuration ...
type mixedConfiguration struct {
	DataDogEnabled bool   `mapstructure:"DD_ENABLE"`
	LogLevel       string `mapstructure:"LOG_LEVEL"`
	Port           int    `env:"PORT"`
	Host           string `env:"HOST, default=my-host"`
	DatabasePort   int    `mapstructure:"DATABASE_PORT" validate:"required"`
}

func TestLoadConfigMixed(t *testing.T) {
	assert := assert.New(t)

	conf := &mixedConfiguration{
		LogLevel:     "ERROR",
		DatabasePort: 5432,
		Port:         8080,
	}

	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("DD_ENABLE", "true")
	t.Setenv("PORT", "8081")
	t.Setenv("HOST", "another-host")

	assert.NoError(LoadConfig(conf))

	assert.Equal("DEBUG", conf.LogLevel)
	assert.True(conf.DataDogEnabled)
	assert.Equal("another-host", conf.Host)
}

func TestLoadConfigWithoutRetag(t *testing.T) {
	assert := assert.New(t)

	conf := &configuration{
		LogLevel:     "ERROR",
		DatabasePort: 5432,
		Port:         8080,
	}

	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("DD_ENABLE", "true")
	t.Setenv("ENV_TAG", "some-value")
	t.Setenv("PORT", "8081")
	t.Setenv("HOST", "another-host")
	t.Setenv("PARAMS", `{"key": "value", "key2": { "nested": { "key" : "value" } } }`)

	assert.NoError(LoadConfigWithoutRetag(conf))

	assert.Equal("DEBUG", conf.LogLevel)
	assert.True(conf.DataDogEnabled)
	assert.Equal("another-host", conf.Host)
}

func TestLoadConfigCheckDefault(t *testing.T) {
	assert := assert.New(t)
	conf := &configuration{
		DatabasePort: 5432,
	}
	assert.NoError(LoadConfigWithoutRetag(conf))
	assert.Equal("my-host", conf.Host)
}

func TestLoadConfigValidation(t *testing.T) {
	conf := &configuration{
		LogLevel: "ERROR",
	}

	assert.Error(t, LoadConfigWithoutRetag(conf))
}

func TestLoadConfigValidationMixed(t *testing.T) {
	conf := &mixedConfiguration{
		LogLevel: "ERROR",
	}

	assert.Error(t, LoadConfig(conf))
}

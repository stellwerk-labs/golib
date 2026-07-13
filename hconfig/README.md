# hconfig

Is a config loading util loading values from environment variables or a config file and validates them according the configuration structure tag.

## Usage

```golang
// Configuration ...
type configuration struct {
 DataDogEnabled bool   `env:"DD_ENABLE"`
 LogLevel       string `env:"LOG_LEVEL"`
 DatabasePort int    `env:"DATABASE_PORT" validate:"required"`

 // "mapstructure" tags were used historically and are still supported, but deprecated.
 LegacyValue string `mapstructure:"LEGACY_VALUE"`
}

func main() {
  conf := &configuration{
    LogLevel: "ERROR",
  }

  err := hconfig.LoadConfig(conf)

  // LoadConfigWithoutRetag allows to load configs that use only `env` keys, without the legacy re-tagging.
  err := hconfig.LoadConfigWithoutRetag(conf)
}
```

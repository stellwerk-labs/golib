# henvflags

[OpenFeature](https://openfeature.dev/) compatible, in-memory feature flag client configured using an environment variable.

```golang
ffProvider := new(featureflags.InMemFeatureFlagProvider)
if cfg.FeatureFlagOverrides != "" {
  if err := ffProvider.AddJsonOverrides(cfg.FeatureFlagOverrides); err != nil {
    sugar.Fatalw("failed to load feature flag overrides: %v", err)
  }
}
_ = openfeature.SetProviderAndWait(ffProvider)
featureFlagClient := openfeature.NewClient("appenv-service")
```

## Pattern support in feature flag criteria

When a match criteria string contains a `re:` prefix, the remainder of the string will be interpreted as a regular 
expression. The pattern must match the entire value.

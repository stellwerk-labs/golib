# hvaultapi

`hvaultapi` provides a way to login and handle token lifecycle management.

Below an example of its usage.

```golang
logw, err := hlogger.NewLogger()
 if err != nil {
  log.Fatalf("Error building logger: %v", err)
 }
 defer hlogger.OnExit(logw.Logger)

logger := logw.Logger

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

// don't use the httpClient here as wrapping modifies it in-place with vault specific settings
vaultHttpClient := vaulttrace.WrapHTTPClient(httptrace.WrapClient(&http.Client{}))
hvaultapiClient, err := hvaultapi.NewWithDefaults(conf.VaultPrefix, conf.VaultTokenPath, conf.VaultRole, vaultHttpClient, logger)
if err != nil {
  sugar.Fatalw("Error initializing vault client", "err", err)
}

hvaultapiClient.WaitUntilReady(ctx)

go hvaultapiClient.PeriodicallyRenewToken(ctx)

vaultApiClient := hvaultapiClient.Client()
```

Code to handle token management has been taken from: [hashicorp/vault-examples](https://github.com/hashicorp/vault-examples/blob/main/examples/token-renewal/go/example.go) repo.

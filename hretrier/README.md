# hretrier

An opinionated wrapper around <https://github.com/justinrixx/retryhttp> for injecting retries into an `http.Client` by modifying the `Transport` field.

The "Standard Retries" configuration is:

- Retry requests when the error is a "dns error" (we failed to resolve the target address so no request was ever made)
- Retry idempotent requests (Get, Head, Delete, Put) when the error is a "timeout error"
- Retry idempotent requests (Get, Head, Delete, Put) when the status code is 500, 503, or 504
- Perform 2 retries
- Use exponential backoff with a cap of 10 seconds

## How to use this

1. Install via `go get -u github.com/stellwerk-labs/golib/hretrier`
2. Then find the places where you configure an http client for SDKs or API clients and wrap the client with `WrapHttpClientWithStandardRetries`.

    - Example of a oapi-codegen client:
      ```
      httpClient := &http.Client{Timeout: time.Second * 10}
      
      // httptrace "gopkg.in/DataDog/dd-trace-go.v1/contrib/net/http"
      httpClient = httptrace.WrapClient(httpClient)
      
      // github.com/stellwerk-labs/golib/hretrier
      httpClient = hretrier.WrapHttpClientWithStandardRetries(httpClient)
      
      credsClient, err := credentials.NewClientWithResponses(
        conf.CredentialsServiceUrl,
        credentials.WithHTTPClient(httpClient),
      )
      // handle error
      ```
      
3. If you want to disable or modify the retry behavior when making a single request, you can use the customization options listed here <https://github.com/justinrixx/retryhttp/blob/main/docs/options.md> to modify the context going into the request.
   
   For example:

   ```
   ctx := retryhttp.SetMaxRetries(context.Background(), 10)
   r, err := credsClient.DoThing(...)
   ```

# hpagination

`hpagination` offers some helper functions to construct / decode page tokens according to the provided arguments.

## Usage

Functions to produce the page token should be used to build API responses header link to the next page.

```go
nextPageQueryValues := url.Values{
  "per_page": []string{strconv.Itoa(params.PerPage)},
  "page":     []string{PackTimeAndIdPageToken(lastTime, lastId)},
}

headers.Link = pagination.FormatNextLinkHeader(requestUrl, nextPageQueryValues)
```

The functions which decode the page token are needed to extract the parameters that define which elements fetch from the DB.

```go
if params.PageToken != "" {
  beforeTime, beforeId, err = UnpackTimeAndIdPageToken(params.PageToken)
  if err != nil {
    return nil, fmt.Errorf("%w: invalid page token", model.ErrBadRequest)
  }
}
```

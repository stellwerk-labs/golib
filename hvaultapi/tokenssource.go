package hvaultapi

type TokenSource interface {
	GetToken() (string, error)
}

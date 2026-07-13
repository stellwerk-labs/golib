package tokens

// TokenSource is the interface for the Vault tokens provider
type TokenSource interface {
	GetToken() (string, error)
}

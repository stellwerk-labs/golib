package tokens

type stringTokenSource struct {
	token string
}

// NewStringTokenSource creates a new instance of a TokenSource that returns the same token every time
func NewStringTokenSource(token string) TokenSource {
	return &stringTokenSource{
		token: token,
	}
}

// GetToken returns the same token every time
func (src *stringTokenSource) GetToken() (string, error) {
	return src.token, nil
}

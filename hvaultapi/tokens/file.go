package tokens

import (
	"fmt"
	"os"
)

type fileTokenSource struct {
	tokenPath string
}

// NewFileTokenSource creates a new instance of a TokenSource that reads Vault token from the local file
func NewFileTokenSource(tokenPath string) TokenSource {
	return &fileTokenSource{
		tokenPath: tokenPath,
	}
}

// GetToken reads Vault token from the local file
func (src *fileTokenSource) GetToken() (string, error) {
	raw, err := os.ReadFile(src.tokenPath)
	if err != nil {
		return "", fmt.Errorf("failed to read token from file %s: %w", src.tokenPath, err)
	}
	return string(raw), nil
}

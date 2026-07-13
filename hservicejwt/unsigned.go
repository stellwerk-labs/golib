package hservicejwt

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

const algorithmNone = "none"
const separator = '.'

type jwtHeader struct {
	Algorithm string `json:"alg"`
}

type unsignedEncoderDecoder struct {
}

func (u *unsignedEncoderDecoder) Encode(jwt ServiceJwt) (string, error) {
	sb := new(strings.Builder)
	raw, _ := json.Marshal(&jwtHeader{Algorithm: algorithmNone})
	sb.WriteString(base64.RawURLEncoding.EncodeToString(raw))
	sb.WriteRune(separator)
	raw, _ = json.Marshal(jwt)
	sb.WriteString(base64.RawURLEncoding.EncodeToString(raw))
	return sb.String(), nil
}

func (u *unsignedEncoderDecoder) Decode(raw string) (ServiceJwt, error) {
	var content ServiceJwt

	// TODO: if we need to support signed JWTs or utilise a common library, then we must switch to using a real
	// 	jwt library!

	parts := strings.Split(raw, string(separator))
	if len(parts) != 2 {
		return content, &ErrInvalidJwt{Inner: errors.Errorf("expected 2 %q-separated parts", separator)}
	}
	var header jwtHeader
	if raw, err := base64.RawURLEncoding.DecodeString(parts[0]); err != nil {
		return content, &ErrInvalidJwt{Inner: errors.WithMessage(err, "invalid header base64")}
	} else if err := json.Unmarshal(raw, &header); err != nil {
		return content, &ErrInvalidJwt{Inner: errors.WithMessage(err, "invalid header json")}
	} else if header.Algorithm != algorithmNone {
		return content, &ErrInvalidJwt{Inner: errors.New("only JWT algorithm 'none' is allowed")}
	} else if raw, err := base64.RawURLEncoding.DecodeString(parts[1]); err != nil {
		return content, &ErrInvalidJwt{Inner: errors.WithMessage(err, "invalid payload base64")}
	} else if err := json.Unmarshal(raw, &content); err != nil {
		return content, &ErrInvalidJwt{Inner: errors.WithMessage(err, "invalid payload json")}
	}

	// validate stateless claims
	if err := content.ValidateStateless(); err != nil {
		return content, err
	}

	return content, nil
}

func DefaultUnsignedEncoderDecoder() EncoderDecoder {
	return &unsignedEncoderDecoder{}
}

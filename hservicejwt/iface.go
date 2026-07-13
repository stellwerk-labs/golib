package hservicejwt

type Encoder interface {
	Encode(jwt ServiceJwt) (string, error)
}

type Decoder interface {
	Decode(raw string) (ServiceJwt, error)
}

type EncoderDecoder interface {
	Encoder
	Decoder
}

type ErrInvalidJwt struct {
	Inner error
}

func (e *ErrInvalidJwt) Error() string {
	return "invalid jwt: " + e.Inner.Error()
}

func (e *ErrInvalidJwt) Unwrap() error {
	return e.Inner
}

type ErrInvalidJwtClaim struct {
	Inner error
}

func (e *ErrInvalidJwtClaim) Error() string {
	return "invalid jwt claim: " + e.Inner.Error()
}

func (e *ErrInvalidJwtClaim) Unwrap() error {
	return e.Inner
}

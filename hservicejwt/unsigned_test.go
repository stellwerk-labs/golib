package hservicejwt

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestEncodeDecodeLoop(t *testing.T) {
	userId := "s-" + uuid.NewString()
	raw, err := DefaultUnsignedEncoderDecoder().Encode(ServiceJwt{
		IssuedAt:      time.Unix(1000, 0),
		ExpiresAt:     time.Unix(1001, 0),
		SubjectUserId: userId,
		DelegatedRoles: map[string]string{
			"/orgs/bananas":                 "member",
			"/orgs/bananas/apps/cats":       "owner",
			"/orgs/bananas/env-types/green": "deployer",
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "eyJhbGciOiJub25lIn0.", raw[:20])
	decoded, err := DefaultUnsignedEncoderDecoder().Decode(raw)
	assert.NoError(t, err)
	assert.Equal(t, userId, decoded.SubjectUserId)
	assert.Equal(t, map[string]string{
		"/orgs/bananas":                 "member",
		"/orgs/bananas/apps/cats":       "owner",
		"/orgs/bananas/env-types/green": "deployer",
	}, decoded.DelegatedRoles)
}

func TestInvalidDecode_parts_few(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("")
	assert.EqualError(t, err, "invalid jwt: expected 2 '.'-separated parts")
}

func TestInvalidDecode_parts_many(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("part1.part2.part3")
	assert.EqualError(t, err, "invalid jwt: expected 2 '.'-separated parts")
}

func TestInvalidDecode_header_base64(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("$$$$$$.apples")
	assert.EqualError(t, err, "invalid jwt: invalid header base64: illegal base64 data at input byte 0")
}

func TestInvalidDecode_header_json(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("000000.apples")
	assert.EqualError(t, err, "invalid jwt: invalid header json: invalid character 'Ó' looking for beginning of value")
}

func TestInvalidDecode_header_algorithm(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("e30.apples")
	assert.EqualError(t, err, "invalid jwt: only JWT algorithm 'none' is allowed")
}

func TestInvalidDecode_payload_base64(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("eyJhbGciOiJub25lIn0.$$$$$$$$")
	assert.EqualError(t, err, "invalid jwt: invalid payload base64: illegal base64 data at input byte 0")
}

func TestInvalidDecode_payload_json(t *testing.T) {
	_, err := DefaultUnsignedEncoderDecoder().Decode("eyJhbGciOiJub25lIn0.000000")
	assert.EqualError(t, err, "invalid jwt: invalid payload json: invalid character 'Ó' looking for beginning of value")
}

func TestDecodeBadUserId(t *testing.T) {
	raw, _ := DefaultUnsignedEncoderDecoder().Encode(ServiceJwt{
		SubjectUserId: "some-user", DelegatedRoles: map[string]string{}, IssuedAt: time.Unix(1000, 0),
		ExpiresAt: time.Unix(1001, 0),
	})
	_, err := DefaultUnsignedEncoderDecoder().Decode(raw)
	assert.EqualError(t, err, "invalid jwt claim: subject user id is not valid")
}

func TestDecodeNoIssuedAt(t *testing.T) {
	raw, _ := DefaultUnsignedEncoderDecoder().Encode(ServiceJwt{
		SubjectUserId: uuid.NewString(), DelegatedRoles: map[string]string{
			"/orgs/bananas": "president",
		},
	})
	_, err := DefaultUnsignedEncoderDecoder().Decode(raw)
	assert.EqualError(t, err, "invalid jwt claim: issued-at time is zero")
}

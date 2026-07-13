package hpagination

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClampOrDefault(t *testing.T) {
	const (
		min          = 1
		max          = 100
		defaultValue = 50
	)
	for _, tc := range []struct {
		Name     string
		Input    *int
		Expected int
	}{
		{"Empty", nil, defaultValue},
		{"Middle", toPtr(25), 25},
		{"Greater", toPtr(125), max},
		{"Less", toPtr(-25), min},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			assert.Equal(t, tc.Expected, ClampOrDefault(tc.Input, min, defaultValue, max))
		})
	}
}

func TestTimeId(t *testing.T) {
	for _, tc := range []struct {
		Name string
		A    time.Time
		B    string
	}{
		{"Empty", time.Time{}, ""},
		{"a-only", time.Now(), ""},
		{"b-only", time.Time{}, "foo"},
		{"both", time.Now(), "world"},
		{"specials", time.Unix(math.MaxInt64>>24, 0), "😀"},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pt := PackTimeAndIdPageToken(tc.A, tc.B)
			a, b, err := UnpackTimeAndIdPageToken(pt)
			assert.NoError(t, err)
			assert.Equal(t, tc.A.UTC(), a)
			assert.Equal(t, tc.B, b)
		})
	}
}

func TestStringString(t *testing.T) {
	for _, tc := range []struct {
		Name, A, B string
	}{
		{"Empty", "", ""},
		{"a-only", "foo", ""},
		{"b-only", "", "foo"},
		{"both", "hello", "world"},
		{"specials", "😀", "😀"},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pt := PackStringString(tc.A, tc.B)
			a, b, err := UnpackStringString(pt)
			assert.NoError(t, err)
			assert.Equal(t, tc.A, a)
			assert.Equal(t, tc.B, b)
		})
	}
}

func TestIntString(t *testing.T) {
	for _, tc := range []struct {
		Name string
		A    int
		B    string
	}{
		{"Empty", 0, ""},
		{"a-only", 42, ""},
		{"b-only", 0, "foo"},
		{"both", 239482394871948, "world"},
		{"specials", math.MaxInt32, "😀"},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pt := PackIntString(tc.A, tc.B)
			a, b, err := UnpackIntString(pt)
			assert.NoError(t, err)
			assert.Equal(t, tc.A, a)
			assert.Equal(t, tc.B, b)
		})
	}
}

func TestPagePerToken(t *testing.T) {
	type ExamplePageToken struct {
		BeforeTime    time.Time `json:"b"`
		BeforeOrgID   string    `json:"o"`
		BeforeID      string    `json:"i"`
		BeforeVersion string    `json:"v"`
	}

	for _, tc := range []struct {
		Name string
		A    interface{}
	}{
		{
			"empty", ExamplePageToken{},
		},
		{"struct", ExamplePageToken{
			BeforeOrgID:   "my-org-id",
			BeforeVersion: "01234",
		},
		},
	} {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			pt, err := PackPageToken(tc.A)
			assert.NoError(t, err)
			var unpackedToken ExamplePageToken
			assert.NoError(t, UnpackPageToken(&unpackedToken, pt))
			assert.Equal(t, tc.A, unpackedToken)
		})
	}
}

func toPtr[T any](v T) *T {
	return &v
}

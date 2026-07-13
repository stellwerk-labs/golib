package hpagination

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// ClampOrDefault will clamp the input if available between the min and max. If it is not provided, it falls back to the
// default.
func ClampOrDefault(input *int, min, def, max int) int {
	t := def
	if input != nil {
		t = *input
	}
	if t > max {
		t = max
	} else if t < min {
		t = min
	}
	return t
}

// FormatNextLinkHeader will return a http link header for the next page, assuming the requestUrl is the original
// request url and the values are the query values to encode.
func FormatNextLinkHeader(requestUrl *url.URL, values url.Values) string {
	nextPageUrl := &url.URL{Path: requestUrl.Path, RawQuery: values.Encode()}
	return fmt.Sprintf("<%s>;rel=\"next\"", nextPageUrl.String())
}

// PackTimeAndIdPageToken constructs a page token that reliably encodes a time and id.
func PackTimeAndIdPageToken(t time.Time, id string) string {
	idBytes := []byte(id)
	token := make([]byte, 0, 16+len(idBytes))
	token = binary.BigEndian.AppendUint64(token, uint64(t.Unix()))
	token = binary.BigEndian.AppendUint64(token, uint64(t.Nanosecond()))
	token = append(token, idBytes...)
	return base64.RawStdEncoding.EncodeToString(token)
}

// UnpackTimeAndIdPageToken unpacks the time and id from a page token packed using PackTimeAndIdPageToken
func UnpackTimeAndIdPageToken(token string) (time.Time, string, error) {
	decodedPageToken, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("failed to decode page token: %v", err)
	}
	if len(decodedPageToken) < 16 {
		return time.Time{}, "", fmt.Errorf("invalid page token: %v", err)
	}
	t := time.Unix(int64(binary.BigEndian.Uint64(decodedPageToken[:8])), int64(binary.BigEndian.Uint64(decodedPageToken[8:16]))).UTC()
	id := string(decodedPageToken[16:])
	return t, id, nil
}

// UnpackIntString unpacks an integer and string from a page token packed using PackIntString
func UnpackIntString(token string) (int, string, error) {
	decodedPageToken, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return 0, "", fmt.Errorf("failed to decode page token: %v", err)
	}
	if len(decodedPageToken) >= 12 {
		i := int(binary.BigEndian.Uint64(decodedPageToken[:8]))
		len1 := int(binary.BigEndian.Uint32(decodedPageToken[8:12]))
		if len1 <= len(decodedPageToken)-12 {
			return i, string(decodedPageToken[12:]), nil
		}
	}
	return 0, "", fmt.Errorf("invalid page token: %v", err)
}

// PackIntString constructs a page token that reliably encodes an integer and string.
func PackIntString(i int, s string) string {
	sb := []byte(s)
	token := make([]byte, 0, 8+4+len(sb))
	token = binary.BigEndian.AppendUint64(token, uint64(i))
	token = binary.BigEndian.AppendUint32(token, uint32(len(sb)))
	token = append(token, sb...)
	return base64.RawStdEncoding.EncodeToString(token)
}

// UnpackStringString unpacks the time and id from a page token packed using PackTimeAndIdPageToken
func UnpackStringString(token string) (string, string, error) {
	decodedPageToken, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return "", "", fmt.Errorf("failed to decode page token: %v", err)
	}
	if len(decodedPageToken) >= 4 {
		len1 := int(binary.BigEndian.Uint32(decodedPageToken[:4]))
		if len1 <= len(decodedPageToken)-4 {
			s1 := string(decodedPageToken[4 : 4+len1])
			s2 := string(decodedPageToken[4+len1:])
			return s1, s2, nil
		}
	}
	return "", "", fmt.Errorf("invalid page token: %v", err)
}

// PackStringString constructs a page token that reliably encodes two strings.
func PackStringString(s1, s2 string) string {
	s1b, s2b := []byte(s1), []byte(s2)
	token := make([]byte, 0, 4+len(s1b)+len(s2b))
	token = binary.BigEndian.AppendUint32(token, uint32(len(s1b)))
	token = append(token, s1b...)
	token = append(token, s2b...)
	return base64.RawStdEncoding.EncodeToString(token)
}

// PackPageToken constructs a page token.
func PackPageToken(i interface{}) (string, error) {
	b, err := json.Marshal(i)
	if err != nil {
		return "", fmt.Errorf("failed to marshal page token: %v", err)
	}
	return base64.RawStdEncoding.EncodeToString(b), nil
}

// UnpackPageToken unpacks a page token
func UnpackPageToken(v interface{}, token string) error {
	decodedPageToken, err := base64.RawStdEncoding.DecodeString(token)
	if err != nil {
		return fmt.Errorf("failed to decode page token: %v", err)
	}
	if err := json.Unmarshal(decodedPageToken, v); err != nil {
		return fmt.Errorf("failed to unmarshal page token: %v", err)
	}
	return nil
}

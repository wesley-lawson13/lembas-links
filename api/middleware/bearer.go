package middleware

import "strings"

// ExtractBearerToken strips a leading "Bearer " prefix from an Authorization
// header value, if present. It's shared by auth.go and rate.go so both key
// off the same token regardless of whether the caller sends the prefix.
func ExtractBearerToken(header string) string {
	if token, ok := strings.CutPrefix(header, "Bearer "); ok {
		return token
	}
	return header
}

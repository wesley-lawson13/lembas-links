package handlers

import (
	"errors"
	"net/url"
	"strings"
)

// maxURLLength caps the length of a target URL we're willing to shorten. Long
// enough for any legitimate deep link, short enough to bound stored/redirected
// payload size.
const maxURLLength = 2048

// validateTargetURL rejects URLs that aren't safe to shorten and redirect to.
// A shortener that 302s to arbitrary input is an open redirector, so we only
// allow absolute http/https URLs with a host and bound their length. This keeps
// out schemes like javascript:, data:, file:, and mailto:.
func validateTargetURL(raw string) error {
	if len(raw) > maxURLLength {
		return errors.New("url too long")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid url")
	}

	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return errors.New("url scheme must be http or https")
	}

	if u.Host == "" {
		return errors.New("url must include a host")
	}

	return nil
}

package client

import (
	"fmt"
	"net/http"
)

// ErrNotFound is returned when the resource is missing (HTTP 404 or equivalent).
type ErrNotFound struct {
	Resource string
}

func (e ErrNotFound) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("%s not found", e.Resource)
	}
	return "resource not found"
}

// HTTPStatusError wraps a non-success HTTP response from VKE or Keystone.
type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e HTTPStatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

func statusErr(code int, body string) error {
	if code == http.StatusNotFound {
		return ErrNotFound{Resource: "resource"}
	}
	return HTTPStatusError{StatusCode: code, Body: body}
}

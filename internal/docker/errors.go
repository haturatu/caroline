package docker

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError preserves the HTTP status returned by the Docker API so callers
// can distinguish terminal resource errors from transient transport failures.
type APIError struct {
	StatusCode int
	Status     string
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("docker API returned %s", e.Status)
	}
	return fmt.Sprintf("docker API returned %s: %s", e.Status, e.Message)
}

func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

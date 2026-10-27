package http1

import (
	"errors"
	"net/http"
)

type HTTPError struct {
	StatusCode int
	Reason     string
}

func (e HTTPError) Error() string {
	return e.Reason
}

func ensureHTTPError(err error, fallbackReason string) HTTPError {
	if e, conv := errors.AsType[HTTPError](err); conv {
		return e
	}
	return HTTPError{StatusCode: http.StatusBadRequest, Reason: fallbackReason}
}

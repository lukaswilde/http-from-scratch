package http1

type HTTPError struct {
	StatusCode int
	Reason     string
}

func (e HTTPError) Error() string {
	return e.Reason
}

package http1

import (
	"bufio"
	"net/http"
	"strings"
)

type CRReader struct {
	reader *bufio.Reader
}

func (r *CRReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func NewCRReader(reader *bufio.Reader) *CRReader {
	return &CRReader{reader: reader}
}

func (r *CRReader) ReadCRLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(line, "\r\n") {
		return "", HTTPError{StatusCode: http.StatusBadRequest, Reason: "line not properly separated by \r\n"}
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

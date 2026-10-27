package http1

import (
	"bufio"
	"net/http"
	"strings"
)

type CRLFReader struct {
	reader *bufio.Reader
}

func (r *CRLFReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func NewCRReader(reader *bufio.Reader) *CRLFReader {
	return &CRLFReader{reader: reader}
}

func (r *CRLFReader) ReadCRLine() (string, error) {
	line, err := r.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(line, "\r\n") {
		return "", HTTPError{StatusCode: http.StatusBadRequest, Reason: "line not properly separated by \r\n"}
	}
	return strings.TrimSuffix(line, "\r\n"), nil
}

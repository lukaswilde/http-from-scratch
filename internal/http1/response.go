package http1

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Response struct {
	header        http.Header
	conn          net.Conn
	buffer        bytes.Buffer
	statusCode    int
	headerWritten bool
}

func NewResponse(conn net.Conn) *Response {
	return &Response{conn: conn, header: http.Header{}, buffer: bytes.Buffer{}, statusCode: http.StatusOK}
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) Write(data []byte) (n int, err error) {
	log.Printf("Trying to write data %q", string(data))

	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	return r.buffer.Write(data)

}

func (r *Response) finishRequest() error {
	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}

	now := time.Now().UTC().Format(http.TimeFormat)

	r.header.Set("Date", now)
	r.header.Set("Content-Length", strconv.Itoa(r.buffer.Len()))

	_, err := fmt.Fprintf(r.conn, "HTTP/1.1 %d %s\r\n", r.statusCode, http.StatusText(r.statusCode))
	if err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	err = r.header.Write(r.conn)
	if err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	_, err = io.WriteString(r.conn, "\r\n")
	if err != nil {
		return fmt.Errorf("failed to write end of headers CRLF: %w", err)
	}

	_, err = r.buffer.WriteTo(r.conn)
	if err != nil {
		return fmt.Errorf("failed to write body: %w", err)
	}

	r.headerWritten = false
	return nil
}

func (r *Response) WriteHeader(statusCode int) {
	if r.headerWritten {
		return
	}
	r.statusCode = statusCode
	r.headerWritten = true

	//}
}

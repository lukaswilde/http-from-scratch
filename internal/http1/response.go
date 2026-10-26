package http1

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Response struct {
	header        http.Header
	conn          net.Conn
	headerWritten bool
}

func NewResponse(conn net.Conn) *Response {
	return &Response{conn: conn, header: http.Header{}}
}

func (r *Response) Header() http.Header {
	return r.header
}

func (r *Response) Write(data []byte) (n int, err error) {
	log.Printf("Trying to write data %q", string(data))

	r.header.Set("Content-Length", strconv.Itoa(len(data)))

	writer := bufio.NewWriter(r.conn)
	defer writer.Flush()

	if !r.headerWritten {
		r.WriteHeader(http.StatusOK)
	}
	return writer.Write(data)
}

func (r *Response) WriteHeader(statusCode int) {
	r.headerWritten = true

	now := time.Now().UTC().Format(http.TimeFormat)
	r.header.Set("Date", now)

	if statusCode != http.StatusOK {
		r.header.Set("Content-Length", "0")
	}

	writer := bufio.NewWriter(r.conn)
	defer writer.Flush()

	var responseString strings.Builder
	responseString.WriteString("HTTP/1.1 " + strconv.Itoa(statusCode) + " " + http.StatusText(statusCode) + "\r\n")
	for k, v := range r.header {
		responseString.WriteString(fmt.Sprintf("%s: %s\r\n", k, v[0]))
	}
	responseString.WriteString("\r\n")
	writer.WriteString(responseString.String())
}

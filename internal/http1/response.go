package http1

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
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

	writer := bufio.NewWriter(r.conn)
	defer writer.Flush()
	writer.Write([]byte("HTTP/1.1 " + strconv.Itoa(statusCode) + "\r\n"))
	for k, v := range r.header {
		writer.Write([]byte(fmt.Sprintf("%s: %s\r\n", k, v[0])))
	}
	writer.Write([]byte("\r\n"))
}

package http1

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
)

func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	request := http.Request{Header: http.Header{}}

	tp := textproto.NewReader(bufio.NewReader(reader))

	// Read and process request line
	requestLine, err := tp.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("failed to read request line: %w", err)
	}

	if err = processRequestLine(requestLine, &request); err != nil {
		return nil, fmt.Errorf("failed to parse request: %w", err)
	}

	// Read and process header lines
	for {
		line, err := tp.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("failed to read header line: %w", err)
		}
		// End of header block
		if line == "" {
			break
		}
		err = processHeaderLine(line, &request)
		if err != nil {
			return nil, fmt.Errorf("failed to parse header: %w", err)
		}
	}

	if err = processBody(reader, &request); err != nil {
		return nil, fmt.Errorf("failed to parse body: %w", err)
	}

	log.Println(request)
	return &request, nil

}

func processBody(reader *bufio.Reader, request *http.Request) error {
	if request.Header.Get("Content-Length") == "" {
		request.ContentLength = 0
		return nil
	}

	contentLength, err := strconv.Atoi(request.Header.Get("Content-Length"))
	if err != nil {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("could not parse Content-Length: %q", request.Header.Get("Content-Length"))}
	}
	request.ContentLength = int64(contentLength)

	body := make([]byte, request.ContentLength)
	bytesRead, err := io.ReadFull(reader, body)

	if err != nil {
		return fmt.Errorf("failed to read body: %w", err)
	}

	if bytesRead != contentLength {
		return HTTPError{http.StatusBadRequest, fmt.Sprintf("expected %d bytes to read, got %d", contentLength, bytesRead)}
	}

	request.Body = io.NopCloser(bytes.NewBuffer(body))
	return nil
}

func processRequestLine(requestLine string, request *http.Request) error {
	fields := strings.Split(requestLine, " ")
	if len(fields) != 3 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("expected 3 fields in request line, got %d", len(fields))}
	}

	// TODO: should we validate these fields? Should Proto be validated for major/minor version
	method := fields[0]
	requestURI := fields[1]
	requestProto := fields[2]

	request.Method = method

	if err := processRequestURI(requestURI, request); err != nil {
		return err
	}

	if err := processHTTPVersion(requestProto, request); err != nil {
		return err
	}

	return nil
}

func processRequestURI(requestURI string, request *http.Request) error {
	if strings.ContainsAny(requestURI, " \r\n\t") {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("request URI contains whitespace: %q", requestURI)}
	}

	// TODO: there are other forms of target URLs that should work, e.g. * for OPTIONS
	requestURL, err := url.Parse(requestURI)
	if err != nil {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("could not parse URI: %q", requestURI)}
	}

	request.RequestURI = requestURI
	request.URL = requestURL

	return nil
}

func processHTTPVersion(requestProto string, request *http.Request) error {
	httpParts := strings.SplitN(requestProto, "/", 2)
	if len(httpParts) != 2 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP version")}
	}
	if httpParts[0] != "HTTP" {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP name")}
	}
	versionParts := strings.SplitN(httpParts[1], ".", 2)
	if len(versionParts) != 2 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP version number")}
	}
	if len(versionParts[0]) > 1 || len(versionParts[1]) > 1 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP version number")}
	}

	majorVersion, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP version number")}
	}
	minorVersion, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("malformed HTTP version number")}
	}

	request.Proto = requestProto
	request.ProtoMajor = majorVersion
	request.ProtoMinor = minorVersion

	return nil
}

func processHeaderLine(headerLine string, request *http.Request) error {
	fields := strings.SplitN(headerLine, ":", 2)
	if len(fields) != 2 {
		return fmt.Errorf("expected 2 strings separated by ':' in header %q, got %d", headerLine, len(fields))
	}

	if fields[0] != strings.TrimSpace(fields[0]) {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("header contains whitespace between header field name and colon: %q", headerLine)}
	}

	// TODO: Should we validate these?
	if fields[0] == "Host" && request.Header.Get("Host") != "" {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Trying to set Host header multiple times")}
	}

	request.Header.Add(fields[0], strings.TrimSpace(fields[1]))

	// TODO: include validation of authority component
	if len(request.Header.Values("Host")) == 0 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Host Header not present in request")}
	}

	return nil
}

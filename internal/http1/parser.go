package http1

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

var acceptedMethods = []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE", "QUERY"}

func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	lineReader := NewCRReader(reader)
	request := &http.Request{Header: make(http.Header), Trailer: make(http.Header)}

	// ------ Request Line --------
	requestLine, err := lineReader.ReadCRLine()
	if err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read request line"}
	}
	if err = processRequestLine(requestLine, request); err != nil {
		return nil, ensureHTTPError(err, "failed to parse request line")
	}

	// ------ Headers --------
	if err = parseFields(lineReader, request.Header); err != nil {
		return nil, err
	}
	if err = validateHeaders(request); err != nil {
		return nil, err
	}

	// ------ Body and Connection state --------
	request.Close = getConnectionClose(request)
	if err = processBody(lineReader, request); err != nil {
		return nil, ensureHTTPError(err, "failed to parse body")
	}

	return request, nil
}

func validateHeaders(request *http.Request) error {
	if len(request.Header.Values("Host")) > 1 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: "Trying to set Host header multiple times"}
	}
	// No Host header valid in HTTP/1.0
	if request.Header.Get("Host") == "" && (request.ProtoMajor != 1 || request.ProtoMinor != 0) {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: "Host header not present in request"}
	}
	request.Host = request.Header.Get("Host")
	return nil
}

func parseFields(reader *CRReader, section http.Header) error {
	for {
		line, err := reader.ReadCRLine()
		if err != nil {
			return ensureHTTPError(err, "failed to read field line")
		}
		if line == "" {
			return nil
		}
		err = processFieldLine(line, section)
		if err != nil {
			return ensureHTTPError(err, "failed to parse field line")
		}
	}
}

func getConnectionClose(request *http.Request) bool {
	insensitiveCompare := func(slice []string, value string) bool {
		return slices.ContainsFunc(slice, func(s string) bool { return strings.EqualFold(s, value) })
	}
	if insensitiveCompare(request.Header.Values("Connection"), "close") {
		return true
	}
	if insensitiveCompare(request.Header.Values("Connection"), "keep-alive") {
		return false
	}
	return request.ProtoMajor == 1 && request.ProtoMinor == 0
}

func processBody(reader *CRReader, request *http.Request) error {
	if te := request.Header.Values("Transfer-Encoding"); len(te) > 0 {
		request.TransferEncoding = te
	}

	if isChunked(request.TransferEncoding) {
		body, err := readChunkedBody(reader, request)
		if err != nil {
			return err
		}
		request.Body = io.NopCloser(bytes.NewReader(body))
		return nil
	}

	contentLength, err := validateContentLength(request.Header)
	if err != nil {
		return err
	}
	request.ContentLength = contentLength

	if contentLength == 0 {
		request.Body = http.NoBody
		return nil
	}

	// TODO: this is not entirely accurate: there might still be compression applied
	body, err := readFixedBody(reader, request)
	if err != nil {
		return err
	}

	request.Body = io.NopCloser(bytes.NewBuffer(body))
	return nil
}

func readFixedBody(reader *CRReader, request *http.Request) ([]byte, error) {
	body := make([]byte, request.ContentLength)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read body"}
	}
	return body, nil
}

func readChunkedBody(reader *CRReader, request *http.Request) ([]byte, error) {
	var body []byte
	for {
		chunkData, err := reader.ReadCRLine()
		if err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read chunk size"}
		}

		// Strip chunk extensions
		if i := strings.IndexRune(chunkData, ';'); i >= 0 {
			chunkData = chunkData[:i]
		}

		chunkSize, err := strconv.ParseInt(chunkData, 16, 64)
		if err != nil || chunkSize < 0 {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "cannot convert chunk size to integer"}
		}

		if chunkSize == 0 {
			break
		}

		chunk := make([]byte, chunkSize)
		if bytesRead, err := io.ReadFull(reader, chunk); err != nil {
			return nil, HTTPError{http.StatusBadRequest, fmt.Sprintf("expected %d bytes to read in chunk, got %d", chunkSize, bytesRead)}
		}
		body = append(body, chunk...)

		// We expect a CRLF after the chunk data
		clrf, err := reader.ReadCRLine()
		if err != nil {
			return nil, HTTPError{http.StatusBadRequest, "unexpected read error after chunk data"}
		}
		if clrf != "" {
			return nil, HTTPError{http.StatusBadRequest, "Expected CRLF after chunk data"}
		}
	}
	// Read and process trailer lines
	if err := parseFields(reader, request.Trailer); err != nil {
		return nil, err
	}

	return body, nil
}

func validateContentLength(header http.Header) (int64, error) {
	contentLengthValues := header.Values("Content-Length")
	if len(contentLengthValues) == 0 {
		return 0, nil
	}
	uniqueValues := slices.Compact(slices.Sorted(slices.Values(contentLengthValues)))
	if len(uniqueValues) > 1 {
		return 0, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("could not parse Content-Length: %q", header.Get("Content-Length"))}
	}

	contentStr := strings.TrimSpace(uniqueValues[0])
	length, err := strconv.ParseInt(contentStr, 10, 64)
	if err != nil || length < 0 {
		return 0, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("invalid content length: %q", contentStr)}
	}

	return length, nil
}

func isChunked(encoding []string) bool {
	if len(encoding) == 0 {
		return false
	}
	lastHeader := encoding[len(encoding)-1]
	return strings.EqualFold(strings.TrimSpace(lastHeader), "chunked")
}

func processRequestLine(requestLine string, request *http.Request) error {
	fields := strings.Split(requestLine, " ")
	if len(fields) != 3 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("expected 3 fields in request line, got %d", len(fields))}
	}

	method := fields[0]
	requestURI := fields[1]
	requestProto := fields[2]

	if !slices.Contains(acceptedMethods, strings.ToUpper(method)) {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("unknown method %q", method)}
	}

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

	if request.ProtoMajor != 1 {
		return HTTPError{StatusCode: http.StatusHTTPVersionNotSupported, Reason: fmt.Sprintf("Only accepting HTTP version 1.x, got (%d, %d)", majorVersion, minorVersion)}
	} else if request.ProtoMinor != 0 && request.ProtoMinor != 1 {
		return HTTPError{StatusCode: http.StatusHTTPVersionNotSupported, Reason: fmt.Sprintf("Only accepting HTTP version 1.x, got (%d, %d)", majorVersion, minorVersion)}
	}

	return nil
}

func processFieldLine(headerLine string, section http.Header) error {
	fields := strings.SplitN(headerLine, ":", 2)
	if len(fields) != 2 {
		return fmt.Errorf("expected 2 strings separated by ':' in header %q, got %d", headerLine, len(fields))
	}

	if fields[0] != strings.TrimSpace(fields[0]) {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("header contains whitespace between header field name and colon: %q", headerLine)}
	}

	section.Add(fields[0], strings.TrimSpace(fields[1]))
	return nil
}

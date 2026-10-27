package http1

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

func ParseRequest(reader *bufio.Reader) (*http.Request, error) {
	request := http.Request{Header: http.Header{}}

	//tp := textproto.NewReader(bufio.NewReader(reader))
	lineReader := NewCRReader(reader)

	// Read and process request line
	//requestLine, err := tp.ReadLine()
	//requestLine, err := reader.ReadString('\n')
	requestLine, err := lineReader.ReadCRLine()
	if err != nil {
		log.Printf("Error reading line: %v", err)
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read request line"}
	}

	if err = processRequestLine(requestLine, &request); err != nil {
		if e, conv := errors.AsType[HTTPError](err); conv {
			return nil, e
		}
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to parse request line"}
	}

	// Read and process header lines
	for {
		line, err := lineReader.ReadCRLine()
		if err != nil {
			if e, conv := errors.AsType[HTTPError](err); conv {
				return nil, e
			}
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read header line"}
		}
		// End of header block
		if line == "" {
			break
		}
		err = processHeaderLine(line, &request.Header)
		if err != nil {
			if e, conv := errors.AsType[HTTPError](err); conv {
				return nil, e
			}
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to parse header line"}
		}
	}
	if len(request.Header.Values("Host")) > 1 {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Trying to set Host header multiple times")}
	}
	if request.Header.Get("Host") == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Host header not present in request")}
	}

	setConnectionClose(&request)

	if err = processBody(lineReader, &request); err != nil {
		if e, conv := errors.AsType[HTTPError](err); conv {
			return nil, e
		}
		return nil, fmt.Errorf("failed to parse body: %w", err)
	}

	log.Println(request)
	return &request, nil

}

func setConnectionClose(request *http.Request) {
	if request.Header.Get("Connection") == "close" {
		request.Close = true
	} else if request.Header.Get("Connection") == "keep-alive" {
		request.Close = false
	} else if request.ProtoMajor == 1 {
		if request.ProtoMinor == 0 {
			request.Close = true
		}
		if request.ProtoMinor == 1 {
			request.Close = false
		}
	} else {
		request.Close = false
	}
}

func processBody(reader *CRReader, request *http.Request) error {
	if len(request.Header.Values("Content-Length")) > 1 {
		checkDuplicates := request.Header.Values("Content-Length")
		slices.Sort(checkDuplicates)
		checkDuplicates = slices.Compact(checkDuplicates)

		if len(checkDuplicates) > 1 {
			return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("could not parse Content-Length: %q", request.Header.Get("Content-Length"))}
		}
	}

	if len(request.Header.Values("Transfer-Encoding")) > 0 {
		request.TransferEncoding = request.Header.Values("Transfer-Encoding")
	}

	if request.Header.Get("Content-Length") == "" {
		request.ContentLength = 0
		request.Header.Set("Content-Length", "0")
	} else {
		contentLength, err := strconv.Atoi(request.Header.Get("Content-Length"))
		if err != nil {
			return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("could not parse Content-Length: %q", request.Header.Get("Content-Length"))}
		}
		request.ContentLength = int64(contentLength)
	}

	if slices.Contains(request.TransferEncoding, "chunked") {
		var body []byte
		for {
			// TODO: here can be chunk extensions
			chunkData, err := reader.ReadCRLine()
			if err != nil {
				return HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read chunk size"}
			}

			if i := strings.IndexRune(chunkData, ';'); i >= 0 {
				chunkData = chunkData[:i]
			}

			bytesToRead, err := strconv.ParseInt(chunkData, 16, 64)
			if err != nil {
				return HTTPError{StatusCode: http.StatusBadRequest, Reason: "cannot convert chunk size to integer"}
			}

			if bytesToRead == 0 {
				// We have hit the end of the chunked body
				break
			}

			chunk := make([]byte, bytesToRead)
			bytesRead, err := io.ReadFull(reader, chunk)
			if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("failed to read body: %w", err)
			}

			if bytesRead != int(bytesToRead) {
				return HTTPError{http.StatusBadRequest, fmt.Sprintf("expected %d bytes to read in chunk, got %d", bytesToRead, bytesRead)}
			}
			body = append(body, chunk...)
			// We expect a CRLF after the chunk data
			line, err := reader.ReadCRLine()
			if line != "" {
				return HTTPError{http.StatusBadRequest, "Expected CRLF after chunk data"}
			}
			if err != nil {
				return HTTPError{http.StatusBadRequest, "unexpected read error after chunk data"}
			}
		}
		// TODO: handle trailer section
		// Read and process trailer lines
		for {
			line, err := reader.ReadCRLine()
			if err != nil {
				if e, conv := errors.AsType[HTTPError](err); conv {
					return e
				}
				return HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to read trailer line"}
			}
			// End of trailer block
			if line == "" {
				break
			}
			if request.Trailer == nil {
				request.Trailer = make(http.Header)
			}
			err = processHeaderLine(line, &request.Trailer)
			if err != nil {
				if e, conv := errors.AsType[HTTPError](err); conv {
					return e
				}
				return HTTPError{StatusCode: http.StatusBadRequest, Reason: "failed to parse trailer line"}
			}
		}

		request.Body = io.NopCloser(bytes.NewBuffer(body))
	} else {
		// TODO: this is not entirely accurate: there might still be compression applied
		body := make([]byte, request.ContentLength)
		bytesRead, err := io.ReadFull(reader, body)

		if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
			return fmt.Errorf("failed to read body: %w", err)
		}

		if bytesRead != int(request.ContentLength) {
			return HTTPError{http.StatusBadRequest, fmt.Sprintf("expected %d bytes to read, got %d", request.ContentLength, bytesRead)}
		}

		request.Body = io.NopCloser(bytes.NewBuffer(body))
	}
	return nil

}

func processRequestLine(requestLine string, request *http.Request) error {
	fields := strings.Split(requestLine, " ")
	if len(fields) != 3 {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("expected 3 fields in request line, got %d", len(fields))}
	}

	method := fields[0]
	requestURI := fields[1]
	requestProto := fields[2]

	acceptedMethods := []string{"HEAD", "GET", "POST", "PUT", "PATCH", "DELETE", "CONNECT", "OPTIONS", "TRACE", "QUERY"}
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

func processHeaderLine(headerLine string, section *http.Header) error {
	fields := strings.SplitN(headerLine, ":", 2)
	if len(fields) != 2 {
		return fmt.Errorf("expected 2 strings separated by ':' in header %q, got %d", headerLine, len(fields))
	}

	if fields[0] != strings.TrimSpace(fields[0]) {
		return HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("header contains whitespace between header field name and colon: %q", headerLine)}
	}

	section.Add(fields[0], strings.ToLower(strings.TrimSpace(fields[1])))
	return nil
}

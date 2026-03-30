package daytripper

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/swedishborgie/daytripper/har"
)

type tripReport struct {
	req     *http.Request
	rsp     *http.Response
	reqBody *streamCopier
	rspBody *streamCopier
	rspErr  error
	entry   *har.Entry
}

func (d *DayTripper) recordTrip(report *tripReport) error {
	report.entry.Time = har.DurationMS(time.Since(report.entry.StartedDateTime))

	d.recordRequest(report)
	d.recordResponse(report)

	if err := d.sendEntry(report.entry); err != nil {
		return err
	}

	return nil
}

func (d *DayTripper) recordRequest(report *tripReport) {
	var reqURL string
	queryString := make([]*har.QueryString, 0)

	if report.req.URL != nil {
		reqURL = report.req.URL.String()

		for k, vs := range report.req.URL.Query() {
			for _, v := range vs {
				queryString = append(queryString, &har.QueryString{
					Name:  k,
					Value: v,
				})
			}
		}
	}

	report.entry.Request = &har.Request{
		Method:      report.req.Method,
		URL:         reqURL,
		HTTPVersion: report.req.Proto,
		Cookies:     convertCookies(report.req.Cookies()),
		Headers:     convertHeaders(report.req.Header),
		QueryString: queryString,
		PostData:    &har.PostData{},
		HeadersSize: headerSize(report.req.Header),
	}

	if report.reqBody != nil {
		report.entry.Request.BodySize = report.reqBody.count
	}

	if report.reqBody != nil {
		if contentType := report.req.Header.Get("Content-Type"); contentType != "" {
			report.entry.Request.PostData.MimeType = contentType
		}

		if utf8.Valid(report.reqBody.buffer.Bytes()) {
			report.entry.Request.PostData.Text = report.reqBody.buffer.String()
		} else {
			report.entry.Request.PostData.Text = base64.StdEncoding.EncodeToString(report.reqBody.buffer.Bytes())
		}

		if report.reqBody.truncated {
			report.entry.Request.PostData.Comment = fmt.Sprintf("body truncated at %d bytes", d.maxBodySize)
		}
	}
}

// headerSize will calculate the size of the headers in their post-parsed state.
// This includes ': ' and the double '\r\n' at the end. Go doesn't expose a way
// to get the header size pre-parsed, so this may not be exact, but it should be
// very close in most cases.
func headerSize(headers http.Header) uint64 {
	var headerSize uint64

	extraPerLine := uint64(len(": ") + len("\r\n"))

	for k, vs := range headers {
		for _, v := range vs {
			headerSize += uint64(len(k)+len(v)) + extraPerLine
		}
	}

	headerSize += uint64(len("\r\n"))

	return headerSize
}

func (d *DayTripper) recordResponse(report *tripReport) {
	if report.rsp == nil {
		// Nothing we can do, no response.
		return
	}

	report.entry.Response = &har.Response{
		Status:      report.rsp.StatusCode,
		StatusText:  report.rsp.Status,
		HTTPVersion: report.rsp.Proto,
		Cookies:     convertCookies(report.rsp.Cookies()),
		Headers:     convertHeaders(report.rsp.Header),
		Content: &har.Content{
			MimeType: report.rsp.Header.Get("Content-Type"),
		},
		HeadersSize: headerSize(report.rsp.Header),
	}

	if report.rspBody != nil {
		bodyBytes := report.rspBody.buffer.Bytes()
		compressedSize := report.rspBody.count
		truncated := report.rspBody.truncated

		if enc := report.rsp.Header.Get("Content-Encoding"); enc != "" {
			if decoded, err := d.bodyDecoder(enc, bodyBytes, d.maxBodySize); err == nil {
				if d.maxBodySize > 0 && int64(len(decoded)) > d.maxBodySize {
					truncated = true
					decoded = decoded[:d.maxBodySize]
				}
				bodyBytes = decoded
			}
		}

		report.entry.Response.BodySize = compressedSize
		report.entry.Response.Content.Size = uint64(len(bodyBytes))
		if uint64(len(bodyBytes)) > compressedSize {
			report.entry.Response.Content.Compression = uint64(len(bodyBytes)) - compressedSize
		}

		if utf8.Valid(bodyBytes) {
			report.entry.Response.Content.Text = string(bodyBytes)
		} else {
			report.entry.Response.Content.Text = base64.StdEncoding.EncodeToString(bodyBytes)
			report.entry.Response.Content.Encoding = "base64"
		}

		if truncated {
			report.entry.Response.Content.Comment = fmt.Sprintf("body truncated at %d bytes", d.maxBodySize)
		}
	}

	if report.rspErr != nil {
		report.entry.Response.Error = report.rspErr.Error()
	}
}

func convertCookies(cookies []*http.Cookie) []*har.Cookie {
	retr := make([]*har.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		retr = append(retr, &har.Cookie{
			Name:     cookie.Name,
			Value:    cookie.Value,
			Path:     cookie.Path,
			Domain:   cookie.Domain,
			Expires:  cookie.Expires,
			Secure:   cookie.Secure,
			HttpOnly: cookie.HttpOnly,
		})
	}

	return retr
}

// DecodeBody attempts to decode a response body according to its Content-Encoding header value.
// Multiple encodings may be listed (comma-separated) and are applied in order. Unknown encodings
// cause the function to return the raw bytes unchanged (best-effort).
// maxSize limits the number of decompressed bytes read; 0 means unlimited. This prevents
// issues with compressed bodies expanding to an enormous decoded body.
func DecodeBody(contentEncoding string, raw []byte, maxSize int64) ([]byte, error) {
	data := raw
	for enc := range strings.SplitSeq(contentEncoding, ",") {
		enc = strings.TrimSpace(strings.ToLower(enc))
		switch enc {
		case "identity", "":
			// no-op
		case "gzip":
			r, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return raw, err
			}
			decoded, err := io.ReadAll(limitReader(r, maxSize))
			if err != nil {
				return raw, err
			}
			data = decoded
		case "deflate":
			// Try zlib (deflate with header) first, then raw deflate.
			zr, err := zlib.NewReader(bytes.NewReader(data))
			if err != nil {
				fr := flate.NewReader(bytes.NewReader(data))
				decoded, err := io.ReadAll(limitReader(fr, maxSize))
				if err != nil {
					return raw, err
				}
				data = decoded
			} else {
				decoded, err := io.ReadAll(limitReader(zr, maxSize))
				if err != nil {
					return raw, err
				}
				data = decoded
			}
		default:
			// Unsupported encoding — return what we have so far unchanged.
			return raw, nil
		}
	}
	return data, nil
}

// limitReader wraps r with an io.LimitReader when maxSize > 0, otherwise returns r unchanged.
// We read maxSize+1 bytes so the caller can detect truncation by checking len(result) > maxSize.
func limitReader(r io.Reader, maxSize int64) io.Reader {
	if maxSize <= 0 {
		return r
	}
	return io.LimitReader(r, maxSize+1)
}

func convertHeaders(headers http.Header) []*har.Header {
	retr := make([]*har.Header, 0, len(headers))
	for header, values := range headers {
		for _, value := range values {
			retr = append(retr, &har.Header{
				Name:  header,
				Value: value,
			})
		}
	}

	return retr
}

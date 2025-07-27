package daytripper

import (
	"encoding/base64"
	"github.com/swedishborgie/daytripper/har"
	"net/http"
	"time"
	"unicode/utf8"
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
	report.entry.Response = &har.Response{
		Status:      report.rsp.StatusCode,
		StatusText:  report.rsp.Status,
		HTTPVersion: report.rsp.Proto,
		Cookies:     convertCookies(report.rsp.Cookies()),
		Headers:     convertHeaders(report.rsp.Header),
		Content: &har.Content{
			Size:     report.rspBody.count,
			MimeType: report.rsp.Header.Get("Content-Type"),
		},
		HeadersSize: headerSize(report.rsp.Header),
		BodySize:    report.rspBody.count,
	}

	if utf8.Valid(report.rspBody.buffer.Bytes()) {
		report.entry.Response.Content.Text = report.rspBody.buffer.String()
	} else {
		report.entry.Response.Content.Text = base64.StdEncoding.EncodeToString(report.rspBody.buffer.Bytes())
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

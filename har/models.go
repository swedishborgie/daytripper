// Package har contains the definitions for the HTTP Archive file format. These definitions are a superset of the
// 1.2 specification, including several Chrome and Firefox specific extensions.
//
// The HAR 1.2 Specification can be found here:
// http://www.softwareishard.com/blog/har-12-spec/
package har

import (
	"encoding/json"
	"time"
)

// HTTPArchive is the main data type for the HAR file format specification.
type HTTPArchive struct {
	Log *Log `json:"log,omitempty"`
}

// Log contains all the information about a recording session.
type Log struct {
	// Version number of the format. If empty, string "1.1" is assumed by default.
	Version string `json:"version"`
	// Creator is the application that created the archive.
	Creator *Agent `json:"creator"`
	// Browser is the application that performed the recorded network activity.
	Browser *Agent `json:"browser,omitempty"`
	// Pages contains a set of all tracked pages. This can be empty.
	Pages []*Page `json:"pages"`
	// Entries contains a set of all tracked network requests.
	Entries []*Entry `json:"entries"`
	// Comment is a comment associated with this log.
	Comment string `json:"comment,omitempty"`
}

// Agent is either the creator or browser in the log file, tracks which software did the recording.
type Agent struct {
	// Name of the application/browser used to export the log.
	Name string `json:"name"`
	// Version of the application/browser used to export the log.
	Version string `json:"version"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

type Page struct {
	// StartedDateTime is the time for the beginning of the page load.
	StartedDateTime time.Time `json:"startedDateTime,omitempty"`
	// ID is a unique identifier of a page within the log. Entries use it to refer to the parent page (see: Entry.PageRef).
	ID string `json:"id,omitempty"`
	// Title is the recorded page title.
	Title string `json:"title,omitempty"`
	// PageTimings contain detailed timing information about how long components of the page took to load.
	PageTimings *PageTimings `json:"pageTimings,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// PageTimings contains information about page load times.
type PageTimings struct {
	// OnContentLoaded is the number of milliseconds since a page load started (Page.StartedDateTime).
	// Use -1 if the timing does not apply to the current request.
	OnContentLoaded DurationMS `json:"onContentLoaded,omitempty"`
	// OnLoad is the number of milliseconds since a page load started (Page.StartedDateTime).
	//Use -1 if the timing does not apply to the current request.
	OnLoad DurationMS `json:"onLoad,omitempty"`
}

// Entry contains information about a single round trip network request.
type Entry struct {
	// PageRef is a reference to the page this request is a part of.
	PageRef string `json:"pageref,omitempty"`
	// StartedDateTime is the time the request was started.
	StartedDateTime time.Time `json:"startedDateTime,omitempty"`
	// Time is the total elapsed time of the request in milliseconds.
	// This is the sum of all timings available in the timings object.
	Time DurationMS `json:"time,omitempty"`
	// Request contains information about the network request from the client.
	Request *Request `json:"request,omitempty"`
	// Response contains informmation about the server response.
	Response *Response `json:"response,omitempty"`
	// Cache contains information about cache usage.
	Cache *Cache `json:"cache"`
	// Timings contains detailed timing information about this network request.
	Timings *Timings `json:"timings,omitempty"`
	// ServerIPAddress is the IP address of the server the client connected to (e.g. the result of DNS resolution by
	// the client).
	ServerIPAddress string `json:"serverIPAddress,omitempty"`
	// Connection is a unique ID of the parent TCP/IP connection, can be the client or server port number.
	// Note that a port number doesn't have to be unique identifier in cases where the port is shared for more
	// connections. If the port isn't available for the application, any other unique connection ID can be used instead
	// (e.g. connection index). Leave out this field if the application doesn't support this info.
	Connection string `json:"connection,omitempty"`
	// Comment is a user defined comment.
	Comment string `json:"comment,omitempty"`

	// Chrome Extensions

	ConnectionID      string              `json:"_connectionId,omitempty"`
	Priority          string              `json:"_priority,omitempty"`
	ResourceType      string              `json:"_resourceType,omitempty"`
	Initiator         *Initiator          `json:"_initiator,omitempty"`
	FromCache         string              `json:"_fromCache,omitempty"`
	WebSocketMessages []*WebSocketMessage `json:"_webSocketMessages,omitempty"`
}

// Initiator tracks which part of a page initiated a specific network request. This is a Chrome specific extension.
type Initiator struct {
	// Type contains what type of object initiated the request.
	Type string `json:"type"`
	// URL tracks which resource initiated the request.
	URL string `json:"url"`
	// LineNumber tracks which line number in the resource initiated the request.
	LineNumber int `json:"lineNumber"`
}

// Request tracks information about a specific network request.
type Request struct {
	// Method is the HTTP request method.
	Method string `json:"method,omitempty"`
	// URL is the absolute URL of the request, not including any request fragments.
	URL string `json:"url,omitempty"`
	// HTTPVersion is the HTTP version used.
	HTTPVersion string `json:"httpVersion,omitempty"`
	// Cookies are the cookies submitted as part of the request.
	Cookies []*Cookie `json:"cookies"`
	// Headers is the headers sent to the server.
	Headers []*Header `json:"headers"`
	// QueryString is the list of query parameter objects.
	QueryString []*QueryString `json:"queryString"`
	// PostData is information about the body of the request.
	PostData *PostData `json:"postData,omitempty"`
	// HeadersSize is the total number of bytes from the start of the HTTP request message until (and including) the
	// double CRLF before the body. Set to -1 if the info is not available.
	HeadersSize uint64 `json:"headersSize,omitempty"`
	// BodySize is the size of the request body (POST data payload) in bytes. Set to -1 if the info is not available.
	BodySize uint64 `json:"bodySize"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// Response contains information about the response to an HTTP request.
type Response struct {
	// Status is the HTTP status code of the response.
	Status int `json:"status,omitempty"`
	// StatusText is the HTTP status text provided by the server.
	StatusText string `json:"statusText,omitempty"`
	// HTTPVersion is the HTTP protocol version used.
	HTTPVersion string `json:"httpVersion,omitempty"`
	// Cookies contains all cookies provided as part of the response.
	Cookies []*Cookie `json:"cookies"`
	// Headers contains all headers sent by the server.
	Headers []*Header `json:"headers"`
	// Content contains the response content.
	Content *Content `json:"content,omitempty"`
	// RedirectURL is the target URL from the Location header (if set).
	RedirectURL string `json:"redirectURL,omitempty"`
	// HeadersSize is the number of bytes from the start of the HTTP response message until (and including) the double
	// CRLF before the body. Set to -1 if the info is not available.
	HeadersSize uint64 `json:"headersSize,omitempty"`
	// BodySize is the size of the received response body in bytes. Set to zero in case of responses coming from the
	// cache (304). Set to -1 if the info is not available.
	BodySize uint64 `json:"bodySize"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`

	// Chrome Extensions

	TransferSize            uint64 `json:"_transferSize,omitempty"`
	Error                   any    `json:"_error,omitempty"`
	FetchedViaServiceWorker bool   `json:"_fetchedViaServiceWorker,omitempty"`
}

// Cookie contains information about a cookie as part of the request or response.
type Cookie struct {
	// Name is the name of the cookie.
	Name string `json:"name,omitempty"`
	// Value is the value of the cookie.
	Value string `json:"value,omitempty"`
	// Path is the path of the cookie.
	Path string `json:"path,omitempty"`
	// Domain is the domain the cookie is associated with.
	Domain string `json:"domain,omitempty"`
	// Expires is the expiration time of the cookie.
	Expires time.Time `json:"expires,omitempty"`
	// Secure indicates whether this cookie can be sent only to TLS endpoints.
	Secure bool `json:"secure,omitempty"`
	// HttpOnly indicates whether this cookie is only accessible to HTTP requests (e.g. no JavaScript).
	HttpOnly bool `json:"httpOnly,omitempty"`
	// Comment is a user defined cookie.
	Comment string `json:"comment,omitempty"`
}

// Header is a header on either the request or the response.
type Header struct {
	// Name is the name of the header.
	Name string `json:"name,omitempty"`
	// Value is the value of the header.
	Value string `json:"value,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// QueryString is the query string as part of the path of a request.
type QueryString struct {
	// Name is the name of the query string pair.
	Name string `json:"name,omitempty"`
	// Value is the value of the query string pair.
	Value string `json:"value,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// PostData is information sent as part of the request in the body. This isn't necessarily just for POST requests.
type PostData struct {
	// MimeType is the mime type of the data.
	MimeType string `json:"mimeType,omitempty"`
	// Params is the posted parameters (for URL encoded requests).
	Params []*PostDataParam `json:"params,omitempty"`
	// Text is the posted data (the format depends on MimeType).
	Text string `json:"text,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// PostDataParam the parsed request, if the request is URLEncoded.
type PostDataParam struct {
	// Name is the name of the POSTed parameter.
	Name string `json:"name,omitempty"`
	// Value is the value of the POSTed parameter.
	Value string `json:"value,omitempty"`
	// FileName is the name of the POSTed file.
	FileName string `json:"fileName,omitempty"`
	// ContentType is the Content type of the POSTed file.
	ContentType string `json:"contentType,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// Content contains information about the body of a response.
type Content struct {
	// Size is the length of the returned content in bytes. Should be equal to response.bodySize if there is no
	// compression and bigger when the content has been compressed.
	Size uint64 `json:"size,omitempty"`
	// Compression is the number of bytes saved. Leave out this field if the information is not available.
	Compression uint64 `json:"compression,omitempty"`
	// MIME type of the response text (value of the Content-Type response header). The charset attribute of the
	// MIME type is included (if available).
	MimeType string `json:"mimeType,omitempty"`
	// Text is the response body sent from the server or loaded from the browser cache. This field is populated with
	// textual content only. The text field is either HTTP decoded text or an encoded (e.g. "base64") representation of
	// the response body. Leave out this field if the information is not available.
	Text string `json:"text,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// Cache contains information about what information was cached.
type Cache struct {
	// BeforeRequest is the state of a cache entry before the request. Leave out this field if the information is not
	// available.
	BeforeRequest *CacheEntry `json:"beforeRequest,omitempty"`
	// AfterRequest is the state of a cache entry after the request. Leave out this field if the information is not
	// available.
	AfterRequest *CacheEntry `json:"afterRequest,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// CacheEntry contains information about a cache state.
type CacheEntry struct {
	// Expires is the expiration time of the cache entry.
	Expires time.Time `json:"expires,omitempty"`
	// LastAccess is the last time the cache entry was opened.
	LastAccess time.Time `json:"lastAccess,omitempty"`
	// ETag is the cache etag associated with the request.
	ETag string `json:"eTag,omitempty"`
	// HitCount is the number of times the cache entry has been opened.
	HitCount uint64 `json:"hitCount,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`
}

// Timings is a structure used to keep track of how long each component of an HTTP request took to execute.
type Timings struct {
	// Time spent in a queue waiting for a network connection.
	Blocked DurationMS `json:"blocked,omitempty"`
	// Time taken to resolve a host name.
	DNS DurationMS `json:"dns,omitempty"`
	// Time taken to create a TCP connection.
	Connect DurationMS `json:"connect,omitempty"`
	// Time taken to send the HTTP request to the server.
	Send DurationMS `json:"send,omitempty"`
	// Waiting for a response from the server.
	Wait DurationMS `json:"wait,omitempty"`
	// Time taken to read the entire response from the server (or cache).
	Receive DurationMS `json:"receive,omitempty"`
	// The time taken to negotiate a TLS connection.
	SSL DurationMS `json:"ssl,omitempty"`
	// Comment is a user provided comment.
	Comment string `json:"comment,omitempty"`

	// Chrome Extensions
	BlockedQueuing           DurationMS `json:"_blocked_queueing,omitempty"`
	WorkerStart              DurationMS `json:"_workerStart,omitempty"`
	WorkerReady              DurationMS `json:"_workerReady,omitempty"`
	WorkerFetchStart         DurationMS `json:"_workerFetchStart,omitempty"`
	WorkerRespondWithSettled DurationMS `json:"_workerRespondWithSettled,omitempty"`
}

// OpCodeType is a websocket opcode type.
type OpCodeType int

const (
	// OpCodeText is a websocket text frame.
	OpCodeText OpCodeType = 1
	// OpCodeBinary is a websocket binary frame.
	OpCodeBinary OpCodeType = 2
)

// WebSocketMessage is a captured websocket frame.
type WebSocketMessage struct {
	// Type is the type of message.
	Type string `json:"type"`
	// Time is the time the frame was sent or received.
	Time TimeMS `json:"time"`
	// OpCode is the frame type.
	OpCode OpCodeType `json:"opcode"`
	// Data is the data for the frame, either UTF-8 for text, or Base64 encoded for binary.
	Data string `json:"data"`
}

// TimeMS wraps time.Time and serializes and unserializes JSON as a float representing the number of milliseconds since
// the Unix Epoch.
type TimeMS time.Time

func (t TimeMS) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(time.Time(t).UnixNano()) / float64(time.Millisecond))
}

func (t *TimeMS) UnmarshalJSON(b []byte) error {
	var tmFloat float64
	if err := json.Unmarshal(b, &tmFloat); err != nil {
		return err
	}

	*t = TimeMS(time.Unix(0, int64(tmFloat*float64(time.Millisecond))))
	return nil
}

// DurationMS wraps time.Duration and serializes and deserializes JSON as a float representing the number of
// milliseconds in the duration.
type DurationMS time.Duration

func (d DurationMS) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(d) / float64(time.Millisecond))
}

func (d *DurationMS) UnmarshalJSON(b []byte) error {
	var durFloat float64
	if err := json.Unmarshal(b, &durFloat); err != nil {
		return err
	}

	*d = DurationMS(durFloat * float64(time.Millisecond))

	return nil
}

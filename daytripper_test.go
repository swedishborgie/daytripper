package daytripper_test

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/swedishborgie/daytripper"
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

type mockTripper struct {
	resp *http.Response
	err  error
}

func (m *mockTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return m.resp, m.err
}

func TestDayTripperBasic(t *testing.T) {
	t.Parallel()

	const expectedBody = "Hello world!"

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(expectedBody)); err != nil {
			t.Error(err)
		}
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})
	tt.Options = append(tt.Options,
		daytripper.WithCreator("test creator"),
		daytripper.WithHARVersion("1.3"),
		daytripper.WithVersion("1.2.3"),
	)
	tt.execute(t)

	if tt.Response.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want %d", tt.Response.StatusCode, http.StatusOK)
		return
	}

	if string(tt.Body) != "Hello world!" {
		t.Fatalf("got %s, want %s", string(tt.Body), "Hello world!")
	}

	if len(tt.Receiver.Entries) != 1 {
		t.Fatalf("got %d, want %d", len(tt.Receiver.Entries), 1)
	}

	entry := tt.Receiver.Entries[0]
	if entry.ServerIPAddress == "" {
		t.Fatalf("expected server IP address to be set")
	}
	if entry.Request.Method != http.MethodGet {
		t.Fatalf("got %s, want %s", entry.Request.Method, http.MethodGet)
	}
	if entry.Response.Content.Text != expectedBody {
		t.Fatalf("got %s, want %s", entry.Response.Content.Text, expectedBody)
	}

	if tt.Receiver.Version.Creator != "test creator" {
		t.Fatalf("got %s, want %s", tt.Receiver.Version.Creator, "test creator")
	}
	if tt.Receiver.Version.Version != "1.2.3" {
		t.Fatalf("got %s, want %s", tt.Receiver.Version.Version, "1.2.3")
	}
	if tt.Receiver.Version.HARVersion != "1.3" {
		t.Fatalf("got %s, want %s", tt.Receiver.Version.HARVersion, "1.3")
	}
}

func TestDayTripperMiddleware(t *testing.T) {
	t.Parallel()

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Error(err)
		}
	}, func(svr *httptest.Server) (*http.Request, error) {
		ctx := daytripper.StartPage(context.Background(), "page_1", "Test Page", "")
		ctx = daytripper.EndPage(ctx, "page_1")
		return http.NewRequestWithContext(ctx, http.MethodGet, svr.URL, nil)
	})
	tt.Options = append(tt.Options, daytripper.WithEntryMiddleware(func(entryReceiver receiver.EntryReceiver) receiver.EntryReceiver {
		return func(entry *har.Entry) error {
			entry.Comment = "Test entry comment"
			return entryReceiver(entry)
		}
	}))
	tt.Options = append(tt.Options, daytripper.WithPageMiddleware(func(pageReceiver receiver.PageReceiver) receiver.PageReceiver {
		return func(page *har.Page) {
			page.Comment = "Test page comment"
			pageReceiver(page)
		}
	}))
	tt.execute(t)

	if tt.Receiver.Entries[0].Comment != "Test entry comment" {
		t.Fatalf("got %s, want %s", tt.Receiver.Entries[0].Comment, "Test entry comment")
	}

	if tt.Receiver.Pages[0].Comment != "Test page comment" {
		t.Fatalf("got %s, want %s", tt.Receiver.Pages[0].Comment, "Test page comment")
	}
}

func TestDayTripperCookies(t *testing.T) {
	t.Parallel()

	var svrURL *url.URL

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:     "cookie3",
			Value:    "value3",
			Path:     "/",
			HttpOnly: true,
		})
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("OK!")); err != nil {
			t.Error(err)
		}
	}, func(svr *httptest.Server) (*http.Request, error) {
		svrURL, _ = url.Parse(svr.URL)
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})
	tt.ReqCookies = []*http.Cookie{
		{
			Name:  "cookie1",
			Value: "value1",
		},
		{
			Name:  "cookie2",
			Value: "value2",
		},
	}
	tt.execute(t)

	if len(tt.Receiver.Entries) != 1 {
		t.Fatalf("got %d, want %d", len(tt.Receiver.Entries), 1)
		return
	}

	entry := tt.Receiver.Entries[0]
	jarCookies := tt.Jar.Cookies(svrURL)

	if len(jarCookies) != 3 {
		t.Fatalf("got %d, want %d", len(jarCookies), 3)
		return
	} else if len(entry.Request.Cookies) != 2 {
		t.Fatalf("got %d, want %d", len(entry.Request.Cookies), 2)
		return
	} else if len(entry.Response.Cookies) != 1 {
		t.Fatalf("got %d, want %d", len(entry.Response.Cookies), 1)
	}

	if entry.Request.Cookies[0].Name != "cookie1" {
		t.Fatalf("got %s, want %s", entry.Request.Cookies[0].Name, "cookie1")
	} else if entry.Request.Cookies[0].Value != "value1" {
		t.Fatalf("got %s, want %s", entry.Request.Cookies[0].Value, "value1")
	}

	if entry.Request.Cookies[1].Name != "cookie2" {
		t.Fatalf("got %s, want %s", entry.Request.Cookies[1].Name, "cookie2")
	} else if entry.Request.Cookies[1].Value != "value2" {
		t.Fatalf("got %s, want %s", entry.Request.Cookies[1].Value, "value2")
	}
}

func TestDayTripperQueryParams(t *testing.T) {
	t.Parallel()

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			t.Error(err)
		}
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL+"?foo=bar&baz", nil)
	})
	tt.execute(t)

	entry := tt.Receiver.Entries[0]

	if len(entry.Request.QueryString) != 2 {
		t.Fatalf("got %d, want %d", len(entry.Request.QueryString), 2)
	}

	qs := entry.Request.QueryString
	idxFoo, idxBaz := 0, 1
	if qs[0].Name == "baz" {
		idxBaz, idxFoo = 0, 1
	}
	if qs[idxFoo].Name != "foo" {
		t.Fatalf("got %s, want %s", qs[0].Name, "foo")
	}
	if qs[idxFoo].Value != "bar" {
		t.Fatalf("got %s, want %s", qs[0].Value, "bar")
	}
	if qs[idxBaz].Name != "baz" {
		t.Fatalf("got %s, want %s", qs[1].Name, "baz")
	}
	if qs[idxBaz].Value != "" {
		t.Fatalf("got %s, want %s", qs[1].Value, "")
	}
}

func TestDayTripperPostBody(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		body     []byte
		mimeType string
		b64Enc   bool
	}{
		{name: "text", body: []byte("This is an example post body."), mimeType: "text/plain; charset=utf-8"},
		{name: "binary", body: []byte{0xff, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, mimeType: "application/octet-stream", b64Enc: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", r.Header.Get("Content-Type"))
				w.WriteHeader(http.StatusOK)
				if _, err := io.Copy(w, r.Body); err != nil {
					t.Error(err)
				}
			}, func(svr *httptest.Server) (*http.Request, error) {
				req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, svr.URL, bytes.NewReader(tc.body))
				if err != nil {
					return nil, err
				}
				req.Header.Set("Content-Type", tc.mimeType)
				return req, nil
			})
			tt.execute(t)

			entry := tt.Receiver.Entries[0]

			if len(tc.body) != int(entry.Request.BodySize) {
				t.Fatalf("got %d, want %d", len(tc.body), int(entry.Request.BodySize))
			}

			if len(tc.body) != int(entry.Response.BodySize) {
				t.Fatalf("got %d, want %d", len(tc.body), int(entry.Response.BodySize))
			}

			if !tc.b64Enc {
				if entry.Request.PostData.Text != string(tc.body) {
					t.Fatalf("got %s, want %s", entry.Request.PostData.Text, tc.body)
				}

				if entry.Response.Content.Text != string(tc.body) {
					t.Fatalf("got %s, want %s", entry.Response.Content.Text, tc.body)
				}
			} else {
				decoded, err := base64.StdEncoding.DecodeString(entry.Request.PostData.Text)
				if err != nil {
					t.Fatal(err)
					return
				}
				if !bytes.Equal(decoded, tc.body) {
					t.Fatalf("got %s, want %s", decoded, tc.body)
				}

				decoded, err = base64.StdEncoding.DecodeString(entry.Response.Content.Text)
				if err != nil {
					t.Fatal(err)
					return
				}
				if !bytes.Equal(decoded, tc.body) {
					t.Fatalf("got %s, want %s", decoded, tc.body)
				}
			}

			if entry.Request.PostData.MimeType != tc.mimeType {
				t.Fatalf("got %s, want %s", entry.Request.PostData.MimeType, tc.mimeType)
			}

			if entry.Response.Content.MimeType != tc.mimeType {
				t.Fatalf("got %s, want %s", entry.Response.Content.MimeType, tc.mimeType)
			}
		})
	}
}

type makeRequestFunc func(*httptest.Server) (*http.Request, error)

type testTrip struct {
	Handler     http.HandlerFunc
	MakeRequest makeRequestFunc
	Response    *http.Response
	Body        []byte
	Receiver    *receiver.MemoryReceiver
	ReqCookies  []*http.Cookie
	Options     []daytripper.Option
	Jar         *cookiejar.Jar
}

func newTestTrip(handler http.HandlerFunc, mkFunc makeRequestFunc) *testTrip {
	return &testTrip{
		Handler:     handler,
		MakeRequest: mkFunc,
	}
}

func (tt *testTrip) execute(t *testing.T) {
	t.Helper()

	svr := httptest.NewTLSServer(tt.Handler)
	client := svr.Client()
	defer svr.Close()

	if tt.ReqCookies != nil {
		jar, err := cookiejar.New(&cookiejar.Options{})
		if err != nil {
			t.Fatal(err)
			return
		}
		client.Jar = jar
		tt.Jar = jar

		svrURL, err := url.Parse(svr.URL)
		if err != nil {
			t.Fatal(err)
			return
		}

		jar.SetCookies(svrURL, tt.ReqCookies)
	}

	recv := receiver.NewMemoryReceiver()
	tt.Options = append(tt.Options, daytripper.WithReceiver(recv), daytripper.WithClient(client))
	dt, err := daytripper.New(tt.Options...)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer func() {
		if err := dt.Close(); err != nil {
			t.Error(err)
		}
	}()

	req, err := tt.MakeRequest(svr)
	if err != nil {
		t.Fatal(err)
		return
	}

	rsp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer func() {
		if err := rsp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	tt.Response = rsp

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		t.Fatal(err)
		return
	}

	tt.Body = body
	tt.Receiver = recv
}

func TestRequestWithError(t *testing.T) {
	t.Parallel()

	mockErr := errors.New("mock error")

	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return nil, mockErr
			},
		},
	}

	recv := receiver.NewMemoryReceiver()
	_, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithClient(client))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Get("https://nowhere.com")
	if !errors.Is(err, mockErr) {
		t.Errorf("got %v, want %v", err, mockErr)
	}
}

func TestRoundTripNilResponseBody(t *testing.T) {
	t.Parallel()

	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{
		resp: &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       nil,
		},
	}
	dt, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithTripper(mt))
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	rsp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	if rsp == nil {
		t.Fatal("expected non-nil response")
	}

	if len(recv.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(recv.Entries))
	}
}

func TestWithIncludeAllFalse(t *testing.T) {
	t.Parallel()

	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	recv := receiver.NewMemoryReceiver()
	client := &http.Client{}
	dt, err := daytripper.New(
		daytripper.WithReceiver(recv),
		daytripper.WithIncludeAll(false),
		daytripper.WithClient(client),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	drainResponse := func(t *testing.T, rsp *http.Response) {
		t.Helper()
		_, _ = io.ReadAll(rsp.Body)
		_ = rsp.Body.Close()
	}

	// Request without IncludeContext — should NOT be recorded.
	req1, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	rsp1, err := client.Do(req1)
	if err != nil {
		t.Fatalf("request 1: %v", err)
	}
	drainResponse(t, rsp1)

	// Request with IncludeContext — should be recorded.
	ctx := daytripper.IncludeContext(context.Background())
	req2, _ := http.NewRequestWithContext(ctx, http.MethodGet, svr.URL, nil)
	rsp2, err := client.Do(req2)
	if err != nil {
		t.Fatalf("request 2: %v", err)
	}
	drainResponse(t, rsp2)

	if len(recv.Entries) != 1 {
		t.Errorf("got %d entries, want 1", len(recv.Entries))
	}
}

func TestDayTripperFlush(t *testing.T) {
	t.Parallel()

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})

	svr := httptest.NewTLSServer(tt.Handler)
	defer svr.Close()

	client := svr.Client()
	recv := receiver.NewMemoryReceiver()
	dt, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, err := tt.MakeRequest(svr)
	if err != nil {
		t.Fatal(err)
	}
	rsp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	if err := dt.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestConcurrentRequests(t *testing.T) {
	t.Parallel()

	const numRequests = 10

	svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	client := svr.Client()
	recv := receiver.NewMemoryReceiver()
	dt, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithClient(client))
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	var wg sync.WaitGroup
	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
			rsp, err := client.Do(req)
			if err != nil {
				t.Errorf("request: %v", err)
				return
			}
			_, _ = io.ReadAll(rsp.Body)
			_ = rsp.Body.Close()
		}()
	}
	wg.Wait()

	if len(recv.Entries) != numRequests {
		t.Errorf("got %d entries, want %d", len(recv.Entries), numRequests)
	}
}

func TestWithTripper(t *testing.T) {
	t.Parallel()

	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{
		resp: &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("hello")),
		},
	}
	dt, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithTripper(mt))
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	rsp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_, _ = io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	if len(recv.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(recv.Entries))
	}
	if recv.Entries[0].Response.Content.Text != "hello" {
		t.Errorf("response text = %q, want %q", recv.Entries[0].Response.Content.Text, "hello")
	}
}

func TestDayTripperContentEncodingGzip(t *testing.T) {
	t.Parallel()

	plaintext := strings.Repeat("This is a gzip-compressed response body. ", 50)

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(plaintext))
		_ = gz.Close()
	}, func(svr *httptest.Server) (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
		if err != nil {
			return nil, err
		}
		// Explicitly set Accept-Encoding so Go's transport does NOT auto-decompress.
		req.Header.Set("Accept-Encoding", "gzip")
		return req, nil
	})
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if entry.Response.Content.Text != plaintext {
		t.Errorf("Content.Text = %q, want %q", entry.Response.Content.Text, plaintext)
	}
	if entry.Response.Content.Size != uint64(len(plaintext)) {
		t.Errorf("Content.Size = %d, want %d", entry.Response.Content.Size, len(plaintext))
	}
	if entry.Response.BodySize >= uint64(len(plaintext)) {
		t.Errorf("BodySize = %d should be less than uncompressed size %d", entry.Response.BodySize, len(plaintext))
	}
	if entry.Response.Content.Compression == 0 {
		t.Errorf("Content.Compression should be non-zero for a compressed response")
	}
}

func TestMaxBodySizeResponseTruncation(t *testing.T) {
	t.Parallel()

	const maxSize = 10
	body := strings.Repeat("A", 100)

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(maxSize))
	tt.execute(t)

	// The caller must still receive the full body.
	if string(tt.Body) != body {
		t.Errorf("caller received %d bytes, want %d", len(tt.Body), len(body))
	}

	entry := tt.Receiver.Entries[0]
	if int64(len(entry.Response.Content.Text)) != maxSize {
		t.Errorf("HAR Content.Text length = %d, want %d", len(entry.Response.Content.Text), maxSize)
	}
	if entry.Response.Content.Comment == "" {
		t.Error("expected a truncation comment on Content, got none")
	}
}

func TestMaxBodySizeGzipTruncation(t *testing.T) {
	t.Parallel()

	// Compress a large plaintext body so the decompressed size >> maxSize.
	plaintext := strings.Repeat("AAAAAAAAAA", 200) // 2000 bytes decompressed
	const maxSize int64 = 50

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte(plaintext))
	_ = gz.Close()

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(compressed.Bytes())
	}, func(svr *httptest.Server) (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
		if err != nil {
			return nil, err
		}
		// Prevent Go's transport from auto-decompressing so we see Content-Encoding.
		req.Header.Set("Accept-Encoding", "gzip")
		return req, nil
	})
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(maxSize))
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if int64(len(entry.Response.Content.Text)) != maxSize {
		t.Errorf("HAR Content.Text length = %d, want %d", len(entry.Response.Content.Text), maxSize)
	}
	if entry.Response.Content.Comment == "" {
		t.Error("expected a truncation comment on Content, got none")
	}
}

func TestMaxBodySizeGzipCallerReadsFullCompressedBody(t *testing.T) {
	t.Parallel()

	plaintext := strings.Repeat("AAAAAAAAAA", 200)
	const maxSize int64 = 50

	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	_, _ = gz.Write([]byte(plaintext))
	_ = gz.Close()
	compressedBytes := compressed.Bytes()

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(compressedBytes)
	}, func(svr *httptest.Server) (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept-Encoding", "gzip")
		return req, nil
	})
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(maxSize))
	tt.execute(t)

	if !bytes.Equal(tt.Body, compressedBytes) {
		t.Errorf("caller received %d bytes, want %d (full compressed body)", len(tt.Body), len(compressedBytes))
	}
}

func TestMaxBodySizeRequestTruncation(t *testing.T) {
	t.Parallel()

	const maxSize = 10
	reqBody := strings.Repeat("B", 100)

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodPost, svr.URL,
			strings.NewReader(reqBody))
	})
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(maxSize))
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if int64(len(entry.Request.PostData.Text)) != maxSize {
		t.Errorf("HAR PostData.Text length = %d, want %d", len(entry.Request.PostData.Text), maxSize)
	}
	if entry.Request.PostData.Comment == "" {
		t.Error("expected a truncation comment on PostData, got none")
	}
}

func TestMaxBodySizeRequestServerReadsFullBody(t *testing.T) {
	t.Parallel()

	const maxSize = 10
	reqBody := strings.Repeat("B", 100)
	var receivedBody []byte
	var readErr error

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, readErr = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodPost, svr.URL,
			strings.NewReader(reqBody))
	})
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(maxSize))
	tt.execute(t)

	if readErr != nil {
		t.Fatalf("server ReadAll: %v", readErr)
	}
	if string(receivedBody) != reqBody {
		t.Errorf("server received %d bytes, want %d", len(receivedBody), len(reqBody))
	}

	entry := tt.Receiver.Entries[0]
	if int64(len(entry.Request.PostData.Text)) != maxSize {
		t.Errorf("HAR PostData.Text length = %d, want %d", len(entry.Request.PostData.Text), maxSize)
	}
	if entry.Request.PostData.Comment == "" {
		t.Error("expected a truncation comment on PostData, got none")
	}
}

func TestMaxBodySizeZeroUnlimited(t *testing.T) {
	t.Parallel()

	body := strings.Repeat("Z", 500)

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})
	// maxBodySize = 0 means unlimited — no truncation.
	tt.Options = append(tt.Options, daytripper.WithMaxBodySize(0))
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if entry.Response.Content.Text != body {
		t.Errorf("HAR Content.Text length = %d, want %d", len(entry.Response.Content.Text), len(body))
	}
	if entry.Response.Content.Comment != "" {
		t.Errorf("expected no truncation comment, got %q", entry.Response.Content.Comment)
	}
}

func TestNewNoReceiver(t *testing.T) {
	t.Parallel()

	dt, err := daytripper.New()
	if !errors.Is(err, daytripper.ErrNoReceiver) {
		t.Errorf("got error %v, want ErrNoReceiver", err)
	}
	if dt != nil {
		t.Error("expected nil DayTripper when receiver is missing")
	}
}

func TestDayTripperFormEncodedBody(t *testing.T) {
	t.Parallel()

	const body = "foo=bar&baz=qux"

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, svr.URL,
			strings.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	})
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if entry.Request.PostData == nil {
		t.Fatal("expected PostData to be set")
	}
	if len(entry.Request.PostData.Params) != 2 {
		t.Fatalf("got %d params, want 2", len(entry.Request.PostData.Params))
	}
	params := make(map[string]string, len(entry.Request.PostData.Params))
	for _, p := range entry.Request.PostData.Params {
		params[p.Name] = p.Value
	}
	if params["foo"] != "bar" {
		t.Errorf("foo = %q, want %q", params["foo"], "bar")
	}
	if params["baz"] != "qux" {
		t.Errorf("baz = %q, want %q", params["baz"], "qux")
	}
}

func TestDayTripperCookieExpires(t *testing.T) {
	t.Parallel()

	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{
			Name:    "session",
			Value:   "abc123",
			Expires: expires,
		})
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
	})
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if len(entry.Response.Cookies) != 1 {
		t.Fatalf("got %d response cookies, want 1", len(entry.Response.Cookies))
	}
	c := entry.Response.Cookies[0]
	if c.Expires == nil {
		t.Fatal("expected cookie Expires to be set, got nil")
	}
	if !c.Expires.Equal(expires) {
		t.Errorf("cookie Expires = %v, want %v", c.Expires, expires)
	}
}

func TestBodyDecoderError(t *testing.T) {
	t.Parallel()

	const rawBody = "not-actually-gzip"
	decodeErr := errors.New("decoder failed")

	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{
		resp: &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Encoding": []string{"gzip"},
				"Content-Type":     []string{"text/plain"},
			},
			Body: io.NopCloser(strings.NewReader(rawBody)),
		},
	}
	dt, err := daytripper.New(
		daytripper.WithReceiver(recv),
		daytripper.WithTripper(mt),
		daytripper.WithBodyDecoder(func(string, io.Reader, io.Writer, int64) error {
			return decodeErr
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	rsp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_, _ = io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	if len(recv.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(recv.Entries))
	}
	// When decoder fails the raw bytes should be used as-is.
	if recv.Entries[0].Response.Content.Text != rawBody {
		t.Errorf("Content.Text = %q, want %q", recv.Entries[0].Response.Content.Text, rawBody)
	}
}

func TestWithBodyDecoder(t *testing.T) {
	t.Parallel()

	const rawBody = "hello"
	const decodedBody = "HELLO"

	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{
		resp: &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Encoding": []string{"custom"},
				"Content-Type":     []string{"text/plain"},
			},
			Body: io.NopCloser(strings.NewReader(rawBody)),
		},
	}
	dt, err := daytripper.New(
		daytripper.WithReceiver(recv),
		daytripper.WithTripper(mt),
		daytripper.WithBodyDecoder(func(_ string, src io.Reader, dst io.Writer, _ int64) error {
			data, err := io.ReadAll(src)
			if err != nil {
				return err
			}
			_, err = io.WriteString(dst, strings.ToUpper(string(data)))
			return err
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	rsp, err := dt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_, _ = io.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	if len(recv.Entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(recv.Entries))
	}
	if recv.Entries[0].Response.Content.Text != decodedBody {
		t.Errorf("Content.Text = %q, want %q", recv.Entries[0].Response.Content.Text, decodedBody)
	}
}

func TestDayTripperRequestBodyNoContentType(t *testing.T) {
	t.Parallel()

	const body = "raw payload"

	tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}, func(svr *httptest.Server) (*http.Request, error) {
		// No Content-Type header set.
		return http.NewRequestWithContext(context.Background(), http.MethodPost, svr.URL,
			strings.NewReader(body))
	})
	tt.execute(t)

	entry := tt.Receiver.Entries[0]
	if entry.Request.PostData == nil {
		t.Fatal("expected PostData to be set")
	}
	if entry.Request.PostData.MimeType != "" {
		t.Errorf("MimeType = %q, want empty string", entry.Request.PostData.MimeType)
	}
	if entry.Request.PostData.Text != body {
		t.Errorf("PostData.Text = %q, want %q", entry.Request.PostData.Text, body)
	}
}

func TestRoundTripDoneFuncErrorNilBody(t *testing.T) {
	t.Parallel()

	recvErr := errors.New("receiver error")
	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{
		resp: &http.Response{
			Status:     "200 OK",
			StatusCode: http.StatusOK,
			Body:       nil,
		},
	}
	dt, err := daytripper.New(
		daytripper.WithReceiver(recv),
		daytripper.WithTripper(mt),
		daytripper.WithEntryMiddleware(func(_ receiver.EntryReceiver) receiver.EntryReceiver {
			return func(*har.Entry) error { return recvErr }
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	_, err = dt.RoundTrip(req)
	if !errors.Is(err, recvErr) {
		t.Errorf("got error %v, want %v", err, recvErr)
	}
}

func TestRoundTripDoneFuncErrorNilResponse(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("transport error")
	recvErr := errors.New("receiver error")
	recv := receiver.NewMemoryReceiver()
	mt := &mockTripper{err: transportErr}
	dt, err := daytripper.New(
		daytripper.WithReceiver(recv),
		daytripper.WithTripper(mt),
		daytripper.WithEntryMiddleware(func(_ receiver.EntryReceiver) receiver.EntryReceiver {
			return func(*har.Entry) error { return recvErr }
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer dt.Close() //nolint:errcheck

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://example.com/", nil)
	_, err = dt.RoundTrip(req)
	if !errors.Is(err, recvErr) {
		t.Errorf("got error %v, want %v", err, recvErr)
	}
}

func TestDayTripperContentEncodingDeflate(t *testing.T) {
	t.Parallel()

	const plaintext = "This is a deflate-compressed response body."

	for _, tc := range []struct {
		name  string
		write func(w io.Writer, data []byte) error
	}{
		{
			name: "zlib",
			write: func(w io.Writer, data []byte) error {
				zw := zlib.NewWriter(w)
				_, err := zw.Write(data)
				_ = zw.Close()
				return err
			},
		},
		{
			name: "raw",
			write: func(w io.Writer, data []byte) error {
				fw, err := flate.NewWriter(w, flate.DefaultCompression)
				if err != nil {
					return err
				}
				_, err = fw.Write(data)
				_ = fw.Close()
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tt := newTestTrip(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.Header().Set("Content-Encoding", "deflate")
				w.WriteHeader(http.StatusOK)
				_ = tc.write(w, []byte(plaintext))
			}, func(svr *httptest.Server) (*http.Request, error) {
				return http.NewRequestWithContext(context.Background(), http.MethodGet, svr.URL, nil)
			})
			tt.execute(t)

			entry := tt.Receiver.Entries[0]
			if entry.Response.Content.Text != plaintext {
				t.Errorf("Content.Text = %q, want %q", entry.Response.Content.Text, plaintext)
			}
			if entry.Response.Content.Size != uint64(len(plaintext)) {
				t.Errorf("Content.Size = %d, want %d", entry.Response.Content.Size, len(plaintext))
			}
		})
	}
}

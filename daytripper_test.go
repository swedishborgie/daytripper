package daytripper_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"github.com/swedishborgie/daytripper"
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
)

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

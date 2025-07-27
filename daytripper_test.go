package daytripper_test

import (
	"context"
	"github.com/swedishborgie/daytripper"
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

type makeRequestFunc func(*httptest.Server) (*http.Request, error)

type testTrip struct {
	Handler     http.HandlerFunc
	MakeRequest makeRequestFunc
	Response    *http.Response
	Body        []byte
	Receiver    *receiver.MemoryReceiver
	ReqCookies  []*http.Cookie
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

	svr := httptest.NewServer(tt.Handler)
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
	dt, err := daytripper.New(daytripper.WithReceiver(recv), daytripper.WithClient(client))
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

package main

import (
	"context"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/swedishborgie/daytripper"
	"github.com/swedishborgie/daytripper/middleware"
	"github.com/swedishborgie/daytripper/receiver"
)

func main() {
	if err := execute(); err != nil {
		log.Fatal(err)
	}
}

func execute() error {
	// Start up a basic web server.
	done, svr, err := setupServer()
	if err != nil {
		return err
	}

	// Set up the DayTripper
	client := http.DefaultClient

	dt, err := daytripper.New(
		daytripper.WithReceiver(receiver.NewHARFileReceiver("log.har")),
		daytripper.WithClient(client),
		daytripper.WithEntryMiddleware(middleware.RedactHeader("Authorization", "******")),
	)
	if err != nil {
		return err
	}
	defer dt.Close()

	// Set up and perform the request with authentication.
	req, err := http.NewRequestWithContext(context.Background(), "GET", "http://"+svr.Addr, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "SuperSecretPassword")

	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	log.Printf("Response: %s", string(body))

	svr.Close()
	<-done
	return nil
}

func setupServer() (chan any, *http.Server, error) {
	svr := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Got authentication: %s", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello world!"))
		}),
	}
	done := make(chan any)
	conn, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, nil, err
	}
	svr.Addr = conn.Addr().String()
	go func() {
		close(done)
		svr.Serve(conn)
	}()

	return done, svr, nil
}

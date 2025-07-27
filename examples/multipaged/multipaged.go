package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"

	"github.com/swedishborgie/daytripper"
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
		daytripper.WithIncludeAll(false), // Don't include all pages, only specific ones we specify.
	)
	if err != nil {
		return err
	}
	defer dt.Close() //nolint:errcheck // This is an example

	for i := 0; i < 10; i++ {
		pageID := fmt.Sprintf("page_%d", i)
		ctx := context.Background()
		ctx = daytripper.IncludeContext(ctx)                                   // Indicate to the recorder we should record this request.
		ctx = daytripper.StartPage(ctx, pageID, fmt.Sprintf("Page %d", i), "") // Start the page.
		ctx = daytripper.Page(ctx, pageID)                                     // Associate this request with the page.
		ctx = daytripper.EndPage(ctx, pageID)                                  // End the page.

		if err := doRequest(ctx, svr.Addr, client); err != nil {
			return err
		}
	}

	if err := svr.Close(); err != nil {
		return err
	}

	<-done
	return nil
}

func doRequest(ctx context.Context, addr string, client *http.Client) error {
	// Set up and perform the request with authentication.
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr, nil)
	if err != nil {
		return err
	}

	rsp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close() //nolint:errcheck // This is an example

	body, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	log.Printf("Response: %s", string(body))
	return nil
}

func setupServer() (chan any, *http.Server, error) {
	pageCount := 0
	svr := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			pageCount++
			log.Printf("Got a request: %d", pageCount)
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "You made request: %d", pageCount) //nolint:errcheck // This is an example
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
		svr.Serve(conn) //nolint:errcheck // This is an example
	}()

	return done, svr, nil
}

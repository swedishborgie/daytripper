package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/swedishborgie/daytripper"
	"github.com/swedishborgie/daytripper/receiver/streaming"
)

func main() {
	if err := execute(); err != nil {
		log.Fatal(err)
	}
}

func execute() error {
	client := http.DefaultClient

	fp, err := os.Create("log.har")
	if err != nil {
		return err
	}
	defer fp.Close() //nolint:errcheck // This is an example

	// streaming.New writes each entry to fp as it arrives rather than buffering them all in memory.
	// The file is finalized (made valid JSON) when dt.Close() is called.
	dt, err := daytripper.New(
		daytripper.WithReceiver(streaming.New(fp)),
		daytripper.WithClient(client),
	)
	if err != nil {
		return err
	}
	defer dt.Close() //nolint:errcheck // This is an example

	req, err := http.NewRequestWithContext(context.Background(), "GET", "https://ifconfig.me/all", nil)
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

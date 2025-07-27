package main

import (
	"context"
	"io"
	"log"
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
	client := http.DefaultClient

	dt, err := daytripper.New(
		daytripper.WithReceiver(receiver.NewHARFileReceiver("log.har")),
		daytripper.WithClient(client),
	)
	if err != nil {
		return err
	}
	defer dt.Close()

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

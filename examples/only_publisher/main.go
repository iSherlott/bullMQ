package main

import (
	"context"
	"log"

	"github.com/iSherlott/bullMQ/examples/only_publisher/app"
)

func main() {
	jobID, err := app.Run(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("published job_id=%s", jobID)
}

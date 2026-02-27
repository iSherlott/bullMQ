package main

import (
	"context"
	"log"

	"github.com/iSherlott/bullMQ/examples/only_consumer/app"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

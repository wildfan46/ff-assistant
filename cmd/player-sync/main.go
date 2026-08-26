package main

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambda"
)

// handler is intentionally a stub for now - just proving the EventBridge
// schedule -> Lambda wiring deploys and fires correctly. The real logic
// (fetch player list from Sleeper, upsert into players table, notify of
// changes) gets built here once the deploy skeleton is confirmed working.
func handler(ctx context.Context) error {
	log.Println("player-sync: hello world - scheduled invocation received")
	return nil
}

func main() {
	lambda.Start(handler)
}

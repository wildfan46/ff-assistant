package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx" database/sql driver
)

var (
	db     *sql.DB
	dbOnce sync.Once
	dbErr  error
)

// getDB lazily opens the connection on the first invocation and reuses it
// across warm Lambda invocations (execution environment reuse).
func getDB(ctx context.Context) (*sql.DB, error) {
	dbOnce.Do(func() {
		connStr, err := fetchConnectionString(ctx)
		if err != nil {
			dbErr = fmt.Errorf("fetch connection string: %w", err)
			return
		}
		conn, err := sql.Open("pgx", connStr)
		if err != nil {
			dbErr = fmt.Errorf("open db: %w", err)
			return
		}
		// Neon serverless + Lambda: keep the pool tiny. Each Lambda
		// execution environment is single-threaded in practice, and
		// Neon's pooled connection endpoint handles the rest.
		conn.SetMaxOpenConns(1)
		conn.SetMaxIdleConns(1)
		db = conn
	})
	return db, dbErr
}

func fetchConnectionString(ctx context.Context) (string, error) {
	paramName := envOrPanic("DATABASE_URL")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("load aws config: %w", err)
	}
	client := ssm.NewFromConfig(cfg)

	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", fmt.Errorf("get parameter %s: %w", paramName, err)
	}
	return *out.Parameter.Value, nil
}

type response struct {
	Message   string `json:"message"`
	DBVersion string `json:"db_version,omitempty"`
	Error     string `json:"error,omitempty"`
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	conn, err := getDB(ctx)
	if err != nil {
		return jsonResponse(500, response{Message: "hello from lambda", Error: err.Error()})
	}

	var version string
	if err := conn.QueryRowContext(ctx, "select version()").Scan(&version); err != nil {
		return jsonResponse(500, response{Message: "hello from lambda", Error: err.Error()})
	}

	return jsonResponse(200, response{Message: "hello from lambda", DBVersion: version})
}

func jsonResponse(status int, body response) (events.APIGatewayProxyResponse, error) {
	b, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}, nil
}

func envOrPanic(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Panicf("missing required env var %s", key)
	}
	return v
}

func main() {
	lambda.Start(handler)
}

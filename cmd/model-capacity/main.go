package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kevingatera/model-capacity/capacity"
)

func readToken(variable string) (string, error) {
	path := os.Getenv(variable)
	if path == "" {
		return "", fmt.Errorf("%s must identify a private token file", variable)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("unable to read private token file for %s", variable)
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 32 {
		return "", fmt.Errorf("token for %s must have at least 32 characters", variable)
	}
	return token, nil
}
func main() {
	reader, err := readToken("READER_TOKEN_FILE")
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	collector, err := readToken("COLLECTOR_TOKEN_FILE")
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	if reader == collector {
		log.Print("reader and collector tokens must differ")
		os.Exit(1)
	}
	endpoint := os.Getenv("COLLECTOR_URL")
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		log.Print("invalid collector URL")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	handler, err := capacity.NewServer(reader, func(ctx context.Context) (capacity.Snapshot, error) {
		return capacity.Fetch(ctx, client, endpoint, collector)
	})
	if err != nil {
		log.Print(err)
		os.Exit(1)
	}
	go handler.Run(ctx, 5*time.Minute)
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8388"
	}
	server := &http.Server{Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		<-ctx.Done()
		shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Print("private allowance service listening")
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Print("HTTP server stopped unexpectedly")
		os.Exit(1)
	}
}

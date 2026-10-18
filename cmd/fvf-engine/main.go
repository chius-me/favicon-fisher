package main

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chius-me/favicon-fisher/internal/engine"
	"github.com/chius-me/favicon-fisher/internal/security"
)

func main() {
	// A hard watchdog also bounds input reading and CPU-bound decoder work.
	timer := time.AfterFunc(35*time.Second, func() { os.Exit(124) })
	defer timer.Stop()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), int(security.MaxJSONBody))
	var request engine.Request
	if !scanner.Scan() || json.Unmarshal(scanner.Bytes(), &request) != nil {
		_ = json.NewEncoder(os.Stdout).Encode(engine.Response{ProtocolVersion: engine.ProtocolVersion, Error: &engine.Failure{Code: "INVALID_REQUEST", Message: "Expected one JSON request line."}})
		return
	}
	// stdin must stay open until the response. EOF means the parent has gone away.
	go func() {
		for scanner.Scan() {
		}
		cancel()
	}()
	response := engine.NewWithFakeIP(request.AllowFakeIP).Handle(ctx, request)
	_ = json.NewEncoder(os.Stdout).Encode(response)
}

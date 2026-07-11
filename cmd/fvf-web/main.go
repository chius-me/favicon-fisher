package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
	"github.com/chius-me/favicon-fisher/internal/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	policy := security.DefaultPolicy
	// Opt-in for trusted private networks only (never enable on the public internet).
	if os.Getenv("FVF_ALLOW_PRIVATE") == "1" {
		policy = security.CLIPolicy
		log.Printf("warning: FVF_ALLOW_PRIVATE=1 — private/loopback fetches are allowed")
	}

	client := security.SafeHTTPClient(security.ClientOptions{
		Timeout: 15 * time.Second,
		Policy:  policy,
	})
	if os.Getenv(security.EnvSigningSecret) == "" {
		log.Printf("warning: %s not set; using ephemeral secret (download tokens invalid after restart)", security.EnvSigningSecret)
	}
	signer := security.NewSigner(os.Getenv(security.EnvSigningSecret), security.DefaultTokenTTL)
	handler := web.NewHandlerWithOptions(web.HandlerOptions{
		Client: client,
		Signer: signer,
		Policy: &policy,
	})
	mux := web.NewMux(handler, web.StaticFS())

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	log.Printf("fvf-web listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

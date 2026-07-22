package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/chius-me/favicon-fisher/internal/security"
	"github.com/chius-me/favicon-fisher/internal/web"
)

func main() {
	security.SetupJSONLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	policy := security.DefaultPolicy
	// Opt-in for trusted private networks only (never enable on the public internet).
	if os.Getenv("FVF_ALLOW_PRIVATE") == "1" {
		policy = security.CLIPolicy
		slog.Warn("FVF_ALLOW_PRIVATE=1 — private/loopback fetches are allowed")
	}

	client := security.SafeHTTPClient(security.ClientOptions{
		Timeout: 15 * time.Second,
		Policy:  policy,
	})
	if os.Getenv(security.EnvSigningSecret) == "" {
		slog.Warn("signing secret not set; using ephemeral secret", "env", security.EnvSigningSecret)
	}
	signer := security.NewSigner(os.Getenv(security.EnvSigningSecret), security.DefaultTokenTTL)
	handler := web.NewHandlerWithOptions(web.HandlerOptions{
		Client: client,
		Signer: signer,
		Policy: &policy,
	})
	limiter := security.RateLimiterFromEnv()
	defer limiter.Stop()
	mux := web.NewMuxWithLimiter(handler, web.StaticFS(), limiter)

	if proxies := os.Getenv(security.EnvTrustedProxies); proxies != "" {
		slog.Info("trusted proxies for client IP", "proxies", proxies)
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("fvf-web listening", "addr", ":"+port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

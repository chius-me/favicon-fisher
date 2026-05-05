package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/chius-me/favicon-fisher/internal/fetcher"
)

func TestRunReturnsErrorWhenURLMissing(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code when URL is missing")
	}
	if !strings.Contains(stderr.String(), "usage") {
		t.Fatalf("expected usage message in stderr, got %q", stderr.String())
	}
}

func TestRunPrintsJSONOnSuccess(t *testing.T) {
	original := fetchFunc
	fetchFunc = func(ctx context.Context, rawURL string, outputDir string) (fetcher.Result, error) {
		return fetcher.Result{
			InputURL:   rawURL,
			PageURL:    "https://example.com",
			IconURL:    "https://example.com/favicon.png",
			OutputPath: outputDir + "/example.com.png",
			Filename:   "example.com.png",
		}, nil
	}
	defer func() { fetchFunc = original }()

	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"--json", "--out", "./tmp", "https://example.com"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d with stderr %q", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "\"icon_url\":\"https://example.com/favicon.png\"") {
		t.Fatalf("expected JSON output with icon_url, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\"output_path\":\"./tmp/example.com.png\"") {
		t.Fatalf("expected JSON output with output_path, got %q", stdout.String())
	}
}

func TestRunPrintsHumanReadableSummary(t *testing.T) {
	original := fetchFunc
	fetchFunc = func(ctx context.Context, rawURL string, outputDir string) (fetcher.Result, error) {
		return fetcher.Result{
			PageURL:    "https://example.com",
			IconURL:    "https://example.com/favicon.png",
			OutputPath: outputDir + "/example.com.png",
		}, nil
	}
	defer func() { fetchFunc = original }()

	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"--out", "./tmp", "https://example.com"}, &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d with stderr %q", exitCode, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "Saved favicon") {
		t.Fatalf("expected success header, got %q", output)
	}
	if !strings.Contains(output, "https://example.com/favicon.png") {
		t.Fatalf("expected icon URL in output, got %q", output)
	}
	if !strings.Contains(output, "./tmp/example.com.png") {
		t.Fatalf("expected output path in output, got %q", output)
	}
}

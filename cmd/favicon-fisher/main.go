package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/chius-me/favicon-fisher/internal/fetcher"
)

var fetchFunc = func(ctx context.Context, rawURL string, outputDir string) (fetcher.Result, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	return fetcher.New(client).Fetch(ctx, rawURL, outputDir)
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("favicon-fisher", flag.ContinueOnError)
	fs.SetOutput(stderr)

	outputDir := fs.String("out", "./out", "output directory")
	jsonOnly := fs.Bool("json", false, "print JSON metadata")

	if err := fs.Parse(args); err != nil {
		printUsage(stderr)
		return 2
	}

	if fs.NArg() != 1 {
		printUsage(stderr)
		return 2
	}

	result, err := fetchFunc(ctx, fs.Arg(0), *outputDir)
	if err != nil {
		fmt.Fprintf(stderr, "favicon-fisher: %v\n", err)
		return 1
	}

	if *jsonOnly {
		encoder := json.NewEncoder(stdout)
		if err := encoder.Encode(result); err != nil {
			fmt.Fprintf(stderr, "favicon-fisher: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}

	fmt.Fprintln(stdout, "Saved favicon")
	fmt.Fprintf(stdout, "  page: %s\n", result.PageURL)
	fmt.Fprintf(stdout, "  icon: %s\n", result.IconURL)
	fmt.Fprintf(stdout, "  file: %s\n", result.OutputPath)
	return 0
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: favicon-fisher [--out DIR] [--json] <url>")
}

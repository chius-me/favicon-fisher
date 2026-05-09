package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/chius-me/favicon-fisher/internal/fetcher"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
)

var (
	outputDir string
	jsonOnly  bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "favicon-fisher [url]",
		Short: "A fast and smart favicon fetcher CLI",
		Long:  `favicon-fisher helps you easily extract and download the best favicon from any website.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var url string
			if len(args) == 0 {
				// Interactive mode if no URL is provided
				if jsonOnly {
					return fmt.Errorf("URL is required when using --json")
				}
				pterm.DefaultBasicText.Println(pterm.LightCyan("Welcome to favicon-fisher! 🎣"))
				pterm.DefaultBasicText.Println("Please enter the website URL you want to fetch the favicon from:")
				fmt.Print("URL > ")
				fmt.Scanln(&url)
				if url == "" {
					return fmt.Errorf("URL cannot be empty")
				}
			} else {
				url = args[0]
			}
			return runFetch(cmd.Context(), url, outputDir, jsonOnly)
		},
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.Flags().StringVarP(&outputDir, "out", "o", "./out", "output directory to save the favicon")
	rootCmd.Flags().BoolVar(&jsonOnly, "json", false, "output JSON metadata only (useful for scripts)")

	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	if err := rootCmd.Execute(); err != nil {
		if !jsonOnly {
			pterm.Error.Println(err.Error())
		} else {
			fmt.Fprintf(os.Stderr, `{"error": "%v"}`+"\n", err)
		}
		os.Exit(1)
	}
}

func runFetch(ctx context.Context, rawURL string, outputDir string, jsonOnly bool) error {
	var spinner *pterm.SpinnerPrinter
	if !jsonOnly {
		spinner, _ = pterm.DefaultSpinner.Start(fmt.Sprintf("Fishing favicon from %s ...", rawURL))
	}

	client := &http.Client{Timeout: 15 * time.Second}
	result, err := fetcher.New(client).Fetch(ctx, rawURL, outputDir)
	
	if err != nil {
		if spinner != nil {
			spinner.Fail("Failed to catch the favicon!")
		}
		return err
	}

	if jsonOnly {
		encoder := json.NewEncoder(os.Stdout)
		if err := encoder.Encode(result); err != nil {
			return fmt.Errorf("encode JSON: %w", err)
		}
		return nil
	}

	if spinner != nil {
		spinner.Success("Got it! 🎣")
	}

	fmt.Println()
	panel := pterm.DefaultBox.WithTitle(pterm.LightGreen("Success")).WithTitleTopLeft().Sprint(
		pterm.Sprintf("%s %s\n%s %s\n%s %s",
			pterm.Cyan("Website: "), result.PageURL,
			pterm.Cyan("Icon URL:"), result.IconURL,
			pterm.Cyan("Saved to:"), pterm.LightMagenta(result.OutputPath),
		),
	)
	pterm.Print(panel)
	
	return nil
}

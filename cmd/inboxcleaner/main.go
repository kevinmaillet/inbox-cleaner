package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/kevinmaillet/inbox-cleaner/internal/config"
	"github.com/kevinmaillet/inbox-cleaner/internal/gmail"
	"github.com/kevinmaillet/inbox-cleaner/internal/scanner"
	"github.com/kevinmaillet/inbox-cleaner/internal/ui"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "scan canceled")
		} else {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printUsage()
		return nil
	}
	if args[0] != "scan" {
		return fmt.Errorf("unknown command %q; only scan is implemented in this milestone", args[0])
	}

	paths, err := config.DefaultPaths()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	credentials := flags.String("credentials", paths.CredentialsFile, "path to Google OAuth client credentials JSON")
	token := flags.String("token", paths.TokenFile, "path for the local OAuth token")
	limit := flags.Int("limit", 10000, "maximum messages to inspect (0 scans all matching messages)")
	concurrency := flags.Int("concurrency", 8, "concurrent Gmail metadata requests")
	query := flags.String("query", "in:anywhere -in:trash -in:spam", "Gmail search query")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("scan does not accept positional arguments")
	}
	if *limit < 0 {
		return fmt.Errorf("--limit cannot be negative")
	}
	if *concurrency < 1 || *concurrency > 64 {
		return fmt.Errorf("--concurrency must be between 1 and 64")
	}
	if err := config.EnsureDirectory(paths.Directory); err != nil {
		return err
	}

	httpClient, err := gmail.Authenticate(ctx, *credentials, *token)
	if err != nil {
		return err
	}
	source := gmail.NewClient(httpClient, *query, *concurrency)
	service := scanner.New(source, *limit)
	items, err := service.Scan(ctx)
	if err != nil {
		return err
	}
	return ui.PrintSubscriptions(os.Stdout, items, time.Now())
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Inbox Cleaner scans Gmail metadata and groups likely subscriptions.

Usage:
  inboxcleaner scan [flags]

Examples:
  inboxcleaner scan
  inboxcleaner scan --limit 10000 --concurrency %d

Run "inboxcleaner scan -h" for scan flags.
`, min(runtime.NumCPU(), 8))
}

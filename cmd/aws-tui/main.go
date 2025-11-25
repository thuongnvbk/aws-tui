package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"aws-tui/internal/aws"
	"aws-tui/internal/config"
	"aws-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

var (
	version = "0.1.0"
)

func main() {
	configPath := flag.String("config", "", "Path to config file (default: ~/.aws-tui/config.yaml)")
	profile := flag.String("profile", "", "AWS profile to use (overrides config)")
	region := flag.String("region", "", "AWS region to use (overrides config)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("aws-tui version %s\n", version)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	setupLogging(cfg.Logging)

	// Determine profile and region
	activeProfile := cfg.DefaultProfile
	if *profile != "" {
		activeProfile = *profile
	}

	activeRegion := "us-east-1"
	if p := cfg.GetProfile(activeProfile); p != nil {
		activeRegion = p.Region
	}
	if *region != "" {
		activeRegion = *region
	}

	// Create AWS client
	ctx := context.Background()
	client, err := aws.NewClient(ctx, activeProfile, activeRegion, cfg.UI.DateFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating AWS client: %v\n", err)
		fmt.Fprintf(os.Stderr, "Make sure your AWS credentials are configured correctly.\n")
		os.Exit(1)
	}

	// Create and run TUI
	model := ui.NewModel(cfg, client)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func setupLogging(cfg config.LoggingConfig) {
	if !cfg.Enabled || cfg.File == "" {
		return
	}

	path := expandPath(cfg.File)
	if path == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("aws-tui ")
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return path
}

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "parser-svc/docs/swagger"
	"parser-svc/internal/app"
)

// @title ParserSvc API
// @version 1.0
// @description ParserSvc reads Structurizr deployment manifests in JSON format and exposes host-centric desired state for Drift Detector.
// @description
// @description MVP behavior:
// @description - reads manifests at startup only
// @description - supports file or directory mode
// @description - parses Structurizr deployment model and keeps desired state in memory
// @BasePath /
// @schemes http
func main() {
	defaultConfigPath := os.Getenv("APP_CONFIG_PATH")
	if defaultConfigPath == "" {
		defaultConfigPath = "./configs/appconfig.Develop.yml"
	}

	configPath := flag.String("config", defaultConfigPath, "path to appconfig yaml")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "parser-svc failed: %v\n", err)
		os.Exit(1)
	}
}

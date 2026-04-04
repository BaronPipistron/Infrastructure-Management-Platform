package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "drift-detector-svc/docs/swagger"
	"drift-detector-svc/internal/app"
)

// @title DriftDetectorSvc API
// @version 1.0
// @description DriftDetectorSvc compares desired and actual host state, detects workload drift, and sends asynchronous reconcile commands.
// @description
// @description MVP supported components:
// @description - node_exporter
// @description - cadvisor
// @BasePath /
// @schemes http
func main() {
	defaultConfigPath := os.Getenv("APP_CONFIG_PATH")
	if defaultConfigPath == "" {
		defaultConfigPath = "./configs/appsettings.Develop.yml"
	}

	configPath := flag.String("config", defaultConfigPath, "path to appsettings yaml")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "drift-detector-svc failed: %v\n", err)
		os.Exit(1)
	}
}

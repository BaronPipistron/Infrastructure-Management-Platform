package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	_ "inventory-svc/docs/swagger"
	"inventory-svc/internal/app"
)

// @title InventorySvc API
// @version 1.0
// @description InventorySvc is a canonical read-model of actual host state with in-memory snapshots and periodic synchronization.
// @description
// @description MVP data sources:
// @description - selfProvisioning bootstrap file
// @description - cAdvisor workloads
// @BasePath /
// @schemes http
func main() {
	defaultConfigPath := os.Getenv("APP_CONFIG_PATH")
	if defaultConfigPath == "" {
		defaultConfigPath = "./appsettings.Develop.yml"
	}

	configPath := flag.String("config", defaultConfigPath, "path to appsettings yaml")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "inventory-svc failed: %v\n", err)
		os.Exit(1)
	}
}

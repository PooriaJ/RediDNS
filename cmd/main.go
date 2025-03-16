package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/PooriaJ/RediDNS/api"
	"github.com/PooriaJ/RediDNS/config"
	"github.com/PooriaJ/RediDNS/db"
	"github.com/PooriaJ/RediDNS/server"
	"github.com/PooriaJ/RediDNS/util"
)

func main() {
	// Initialize logger
	logger := util.NewLogger()
	logger.Info("Starting DNS Server")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
	}

	// Create context that can be canceled
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize Redis connection
	redisClient, err := db.NewRedisClient(ctx, cfg)
	if err != nil {
		logger.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize MariaDB connection
	mariadbClient, err := db.NewMariaDBClient(cfg)
	if err != nil {
		logger.Fatalf("Failed to connect to MariaDB: %v", err)
	}
	defer mariadbClient.Close()

	// Initialize database schema
	if err := mariadbClient.InitSchema(); err != nil {
		logger.Fatalf("Failed to initialize database schema: %v", err)
	}

	// Initialize DNS server
	dnsServer, err := server.NewDNSServer(cfg, redisClient, mariadbClient, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize DNS server: %v", err)
	}

	// Start DNS server in a goroutine
	go func() {
		if err := dnsServer.Start(); err != nil {
			logger.Fatalf("Failed to start DNS server: %v", err)
		}
	}()

	// Initialize and start API server
	apiServer := api.NewAPIServer(cfg, redisClient, mariadbClient, logger)
	go func() {
		if err := apiServer.Start(); err != nil {
			logger.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	// Shutdown servers gracefully
	logger.Info("Shutting down servers...")
	dnsServer.Stop()
	apiServer.Stop()

	logger.Info("Server shutdown complete")
}

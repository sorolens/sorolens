package main

import (
	"fmt"
	"log"
	"regexp"

	"github.com/sorolens/sorolens/apps/api/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Print the configuration values to the console for debugging purposes
	fmt.Println("sorolens/api starting")
	fmt.Printf("  port:                  %s\n", cfg.Port)
	fmt.Printf("  database_url:          %s\n", maskPassword(cfg.DatabaseURL))
	fmt.Printf("  redis_url:             %s\n", cfg.RedisURL)
	fmt.Printf("  soroban_rpc_url:       %s\n", cfg.SorobanRPCURL)
	fmt.Printf("  stellar_network:       %s\n", cfg.StellarNetwork)
	fmt.Printf("  log_level:             %s\n", cfg.LogLevel)
	fmt.Printf("  indexer_poll_interval: %s\n", cfg.IndexerPollInterval)
	fmt.Printf("  indexer_ledger_window: %d\n", cfg.IndexerLedgerWindow)
	fmt.Printf("  indexer_max_duration:  %s\n", cfg.IndexerMaxDuration)
}

var passwordRe = regexp.MustCompile(`(://[^:]+:)([^@]+)(@)`)

func maskPassword(dsn string) string {
	return passwordRe.ReplaceAllString(dsn, "${1}***${3}")
}

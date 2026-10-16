package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/services/indexer/internal/sorobanrpc"
)

// indexedNetworks are the networks that can each get their own RPC endpoint
// through SOROBAN_RPC_URL_<NETWORK>.
var indexedNetworks = []string{"testnet", "mainnet", "futurenet"}

// buildRPCClients builds one Soroban RPC client per configured network.
//
//   - SOROBAN_RPC_URL_TESTNET / _MAINNET / _FUTURENET point a network at its
//     own node.
//   - SOROBAN_RPC_URL is the default client, keyed by "", that the poller
//     falls back to for contracts on a network without a dedicated URL.
//   - SOROBAN_RPC_TIMEOUT (Go duration, default 30s) bounds each request and
//     SOROBAN_RPC_MAX_RETRIES (default 3) bounds retries of HTTP 429/5xx and
//     transport failures.
//
// At least one URL must be set: indexing without a node can only fail, so the
// indexer refuses to start rather than run passes that do nothing.
func buildRPCClients(getenv func(string) string) (map[string]*sorobanrpc.Client, error) {
	var opts []sorobanrpc.Option
	if v := getenv("SOROBAN_RPC_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("SOROBAN_RPC_TIMEOUT=%q: want a positive Go duration such as 30s", v)
		}
		opts = append(opts, sorobanrpc.WithTimeout(d))
	}
	if v := getenv("SOROBAN_RPC_MAX_RETRIES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("SOROBAN_RPC_MAX_RETRIES=%q: want a non-negative integer", v)
		}
		opts = append(opts, sorobanrpc.WithMaxRetries(n))
	}

	clients := make(map[string]*sorobanrpc.Client)
	for _, network := range indexedNetworks {
		if url := strings.TrimSpace(getenv("SOROBAN_RPC_URL_" + strings.ToUpper(network))); url != "" {
			clients[network] = sorobanrpc.New(url, opts...)
		}
	}
	if url := strings.TrimSpace(getenv("SOROBAN_RPC_URL")); url != "" {
		clients[""] = sorobanrpc.New(url, opts...)
	}
	if len(clients) == 0 {
		return nil, errors.New("no Soroban RPC endpoint configured: set SOROBAN_RPC_URL or SOROBAN_RPC_URL_<NETWORK> (TESTNET, MAINNET, FUTURENET)")
	}
	return clients, nil
}

// networkLabel names the default client in logs.
func networkLabel(network string) string {
	if network == "" {
		return "default"
	}
	return network
}

package main

import (
	"strings"
	"testing"
)

func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestBuildRPCClients_PerNetworkAndDefault(t *testing.T) {
	clients, err := buildRPCClients(envMap(map[string]string{
		"SOROBAN_RPC_URL_TESTNET": "https://testnet.example",
		"SOROBAN_RPC_URL_MAINNET": " https://mainnet.example ",
		"SOROBAN_RPC_URL":         "https://default.example",
	}))
	if err != nil {
		t.Fatalf("buildRPCClients: %v", err)
	}
	want := map[string]string{
		"testnet": "https://testnet.example",
		"mainnet": "https://mainnet.example",
		"":        "https://default.example",
	}
	if len(clients) != len(want) {
		t.Fatalf("got %d clients, want %d", len(clients), len(want))
	}
	for network, url := range want {
		c, ok := clients[network]
		if !ok || c.Endpoint() != url {
			t.Errorf("network %q: got %v, want endpoint %s", network, c, url)
		}
	}
}

func TestBuildRPCClients_RequiresAnEndpoint(t *testing.T) {
	_, err := buildRPCClients(envMap(nil))
	if err == nil || !strings.Contains(err.Error(), "SOROBAN_RPC_URL") {
		t.Fatalf("want a missing-endpoint error naming SOROBAN_RPC_URL, got %v", err)
	}
}

func TestBuildRPCClients_RejectsBadTuning(t *testing.T) {
	for _, env := range []map[string]string{
		{"SOROBAN_RPC_URL": "http://x", "SOROBAN_RPC_TIMEOUT": "soon"},
		{"SOROBAN_RPC_URL": "http://x", "SOROBAN_RPC_TIMEOUT": "-1s"},
		{"SOROBAN_RPC_URL": "http://x", "SOROBAN_RPC_MAX_RETRIES": "-2"},
	} {
		if _, err := buildRPCClients(envMap(env)); err == nil {
			t.Errorf("env %v: want error", env)
		}
	}
	if _, err := buildRPCClients(envMap(map[string]string{
		"SOROBAN_RPC_URL": "http://x", "SOROBAN_RPC_TIMEOUT": "5s", "SOROBAN_RPC_MAX_RETRIES": "0",
	})); err != nil {
		t.Fatalf("valid tuning rejected: %v", err)
	}
}

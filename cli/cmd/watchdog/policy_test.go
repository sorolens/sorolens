package watchdog

import (
	"strings"
	"testing"
)

const (
	testMonitored = "CAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAD2KM"
)

func TestBuildPolicyGetArgsRequiresSource(t *testing.T) {
	if _, err := buildPolicyGetArgs(testMonitored, testMonitored, "", "testnet", "r", "p", "soroban"); err == nil {
		t.Fatal("expected error without source account")
	}
}

func TestBuildPolicyGetArgsRejectsBadContract(t *testing.T) {
	if _, err := buildPolicyGetArgs("nope", testMonitored, "alice", "testnet", "r", "p", "soroban"); err == nil {
		t.Fatal("expected invalid contract id error")
	}
}

func TestBuildPolicyGetArgsIsReadOnly(t *testing.T) {
	argv, err := buildPolicyGetArgs(testMonitored, testMonitored, "alice", "testnet", "r", "p", "soroban")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(argv, " ")
	for _, want := range []string{"--send=no", "-- get_policy --contract_id " + testMonitored} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv %q missing %q", joined, want)
		}
	}
}

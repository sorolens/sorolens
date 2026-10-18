package cmd

import "testing"

func TestLabelCommandArguments(t *testing.T) {
	if labelCmd.Use != "label <name> <account-or-contract-id>" {
		t.Fatalf("unexpected usage: %q", labelCmd.Use)
	}
	if err := labelCmd.Args(labelCmd, []string{"treasury"}); err == nil {
		t.Fatal("label command accepted one argument")
	}
	if err := labelCmd.Args(labelCmd, []string{"treasury", "G"}); err != nil {
		t.Fatalf("label command rejected two arguments: %v", err)
	}
}
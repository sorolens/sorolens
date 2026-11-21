package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sorolens/sorolens/cli/internal/client"
	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// runInspectTx implements "inspect --tx <hash> [contract-id]".
func runInspectTx(cmd *cobra.Command, rawHash string, args []string) error {
	if globalConfig.NoColor {
		_ = os.Setenv("NO_COLOR", "1")
	}
	txHash := strings.ToLower(strings.TrimSpace(rawHash))
	if !txHashPattern.MatchString(txHash) {
		return fmt.Errorf("invalid transaction hash %q: expected 64 hexadecimal characters", rawHash)
	}

	c := newClient(globalConfig.Timeout)
	stop := startSpinner("Fetching transaction...")

	var contractID string
	if len(args) == 1 {
		contractID = args[0]
	} else {
		trace, err := c.GetInvocationTrace(cmd.Context(), txHash)
		if err != nil {
			stop()
			return txError(err, txHash, "")
		}
		if trace.Root != nil {
			contractID = trace.Root.ContractID
		}
		if contractID == "" {
			stop()
			return fmt.Errorf("transaction %s has no recorded invocation, so its contract is unknown: pass the contract ID as well", txHash)
		}
	}

	detail, err := c.GetInvocation(cmd.Context(), contractID, txHash)
	stop()
	if err != nil {
		return txError(err, txHash, contractID)
	}

	if globalConfig.JSON {
		if detail.Events == nil {
			detail.Events = []client.Event{}
		}
		return format.PrintJSON(detail)
	}

	pairs := [][]string{
		{"Tx Hash", detail.TxHash},
		{"Contract ID", detail.ContractID},
		{"Function", detail.FunctionName},
		{"Status", detail.Status},
		{"Ledger", fmt.Sprintf("%d", detail.Ledger)},
		{"Closed At", detail.LedgerClosedAt.Format(time.RFC3339)},
		{"Resource Fee", fmt.Sprintf("%d", detail.ResourceFeeCharged)},
		{"CPU Instructions", fmt.Sprintf("%d", detail.CPUInsn)},
		{"Memory Bytes", fmt.Sprintf("%d", detail.MemByte)},
		{"Ledger Read Bytes", fmt.Sprintf("%d", detail.LedgerReadByte)},
		{"Ledger Write Bytes", fmt.Sprintf("%d", detail.LedgerWriteByte)},
	}
	fmt.Println(format.RenderKeyValue(pairs))

	if len(detail.Events) == 0 {
		fmt.Println("\nNo events emitted.")
		return nil
	}
	rows := make([][]string, len(detail.Events))
	for i, e := range detail.Events {
		rows[i] = []string{e.Type, e.ContractID, fmt.Sprintf("%v", e.TopicDecoded), fmt.Sprintf("%v", e.ValueDecoded)}
	}
	fmt.Printf("\nEvents (%d)\n%s\n", len(detail.Events), format.RenderTable([]string{"Type", "Contract", "Topics", "Value"}, rows))
	return nil
}

// txError turns API failures into messages that say what was not found.
func txError(err error, txHash, contractID string) error {
	var se *client.SorolensError
	if errors.As(err, &se) {
		switch se.Status {
		case 400:
			return fmt.Errorf("invalid transaction hash %q: %s", txHash, se.Message)
		case 404:
			if contractID != "" {
				return fmt.Errorf("transaction %s not found for contract %s", txHash, contractID)
			}
			return fmt.Errorf("transaction not found: %s", txHash)
		}
	}
	return err
}

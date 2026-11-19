package watchdog

import (
	"errors"
	"fmt"
	"os"

	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// policyFuncName is the read-only watchdog entry point (contracts/watchdog).
const policyFuncName = "get_policy"

var (
	policyWatchdogContract string
	policySourceAccount    string
	policyNetwork          string
	policyRPCURL           string
	policyPassphrase       string
	policySorobanBin       string
	policyDryRun           bool
)

var policyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Read on-chain monitoring policies",
}

var policyGetCmd = &cobra.Command{
	Use:   "get <monitored-contract-id>",
	Short: "Read the monitoring policy stored on-chain for a contract",
	Long: `Read the monitoring policy of a monitored contract by simulating the
watchdog contract's read-only get_policy entry point through the soroban CLI.
Nothing is signed or submitted.

Policies are written by the watchdog admin with set_policy (see
docs/watchdog-policy.md); this command is read-only.`,
	Example: `  sorolens watchdog policy get CACX... --watchdog-contract-id CAC2... \
    --source-account GABC...`,
	Args:         cobra.ExactArgs(1),
	RunE:         runPolicyGet,
	SilenceUsage: true,
}

func init() {
	f := policyGetCmd.Flags()
	f.StringVar(&policyWatchdogContract, "watchdog-contract-id", "", "Watchdog contract id (C...); defaults to $SOROLENS_WATCHDOG_CONTRACT_ID, then $WATCHDOG_CONTRACT_ID")
	f.StringVar(&policySourceAccount, "source-account", "", "Public account address (G...) or soroban CLI key alias used to simulate the read")
	f.StringVar(&policyNetwork, "network", "testnet", "Stellar network (testnet, mainnet, futurenet)")
	f.StringVar(&policyRPCURL, "rpc-url", defaultRPCURL, "Soroban RPC endpoint")
	f.StringVar(&policyPassphrase, "network-passphrase", defaultPassphrase, "Network passphrase")
	f.StringVar(&policySorobanBin, "soroban-bin", "soroban", "Path to the soroban CLI binary")
	f.BoolVar(&policyDryRun, "dry-run", false, "Print the soroban CLI invocation without executing it")
	policyCmd.AddCommand(policyGetCmd)
}

// NewPolicyCommand returns the `watchdog policy` command group.
func NewPolicyCommand() *cobra.Command {
	return policyCmd
}

// buildPolicyGetArgs validates inputs and assembles the soroban CLI argument
// vector for a read-only get_policy simulation.
func buildPolicyGetArgs(monitored, watchdog, source, network, rpc, passphrase, bin string) ([]string, error) {
	if err := validateContractID(monitored); err != nil {
		return nil, err
	}
	if err := validateContractID(watchdog); err != nil {
		return nil, err
	}
	if source == "" {
		return nil, errors.New("--source-account is required (a G... address or a soroban CLI key alias)")
	}
	return []string{
		bin, "contract", "invoke",
		"--id", watchdog,
		"--network", network,
		"--rpc-url", rpc,
		"--network-passphrase", passphrase,
		"--source-account", source,
		"--send=no",
		"--",
		policyFuncName,
		"--contract_id", monitored,
	}, nil
}

func runPolicyGet(cmd *cobra.Command, args []string) error {
	watchdog := policyWatchdogContract
	if watchdog == "" {
		watchdog = os.Getenv("SOROLENS_WATCHDOG_CONTRACT_ID")
	}
	if watchdog == "" {
		watchdog = os.Getenv("WATCHDOG_CONTRACT_ID")
	}
	argv, err := buildPolicyGetArgs(args[0], watchdog, policySourceAccount, policyNetwork, policyRPCURL, policyPassphrase, policySorobanBin)
	if err != nil {
		return err
	}
	if policyDryRun {
		fmt.Println(shellQuoteArgs(argv))
		return nil
	}
	out, err := runSoroban(cmd.Context(), argv[0], argv[1:])
	if err != nil {
		return err
	}
	// The soroban CLI prints the contract's return value as JSON; "null"
	// means no policy is set.
	if string(trimSpace(out)) == "null" || len(trimSpace(out)) == 0 {
		fmt.Fprintf(os.Stderr, "No policy set for %s\n", args[0])
		return nil
	}
	if jsonOut, _ := cmd.Flags().GetBool("json"); jsonOut {
		fmt.Println(string(trimSpace(out)))
		return nil
	}
	fmt.Println(format.RenderKeyValue([][]string{{"Contract ID", args[0]}, {"Policy (JSON)", string(trimSpace(out))}}))
	return nil
}

func trimSpace(b []byte) []byte {
	i, j := 0, len(b)
	for i < j && (b[i] == ' ' || b[i] == '\n' || b[i] == '\r' || b[i] == '\t') {
		i++
	}
	for j > i && (b[j-1] == ' ' || b[j-1] == '\n' || b[j-1] == '\r' || b[j-1] == '\t') {
		j--
	}
	return b[i:j]
}

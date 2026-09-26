package watchdog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sorolens/sorolens/cli/internal/format"
	"github.com/spf13/cobra"
)

// registerFuncName is the watchdog contract entry point invoked by this
// command (contracts/watchdog/src/lib.rs).
const registerFuncName = "register_contract"

// defaultRPCURL and defaultPassphrase mirror the values used by
// contracts/watchdog/scripts/deploy-testnet.sh and the repository .env.example.
const (
	defaultRPCURL     = "https://soroban-testnet.stellar.org:443"
	defaultPassphrase = "Test SDF Network ; September 2015"
)

var registerCmd = &cobra.Command{
	Use:   "register <monitored-contract-id>",
	Short: "Register a contract for monitoring via the watchdog contract",
	Long: `Register a contract for monitoring by invoking the watchdog contract's
register_contract entry point on-chain.

<monitored-contract-id> is the 'C...' address of the contract to monitor.
--name is stored as a Soroban Symbol (letters, digits and underscores, at
most 9 characters) and --interval is the expected health-check cadence
(e.g. 300s, 5m, 1h).

Credentials, in decreasing precedence:
  1. --secret-key or --source-account flag
  2. SOROLENS_WATCHDOG_SECRET environment variable
  3. a named account configured in the soroban CLI (--soroban-account)

The watchdog contract id comes from --watchdog-contract-id, the
SOROLENS_WATCHDOG_CONTRACT_ID environment variable, or the repository's
WATCHDOG_CONTRACT_ID.

Use --dry-run to print the soroban CLI invocation without executing it.`,
	Example: `  # Preview the invocation without sending it
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --dry-run

  # Register on testnet using a secret key
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --source-account SABC...

  # Use a soroban CLI key alias instead of a raw secret
  sorolens watchdog register CACX... --name my_app --interval 300s \
    --watchdog-contract-id CAC2... --soroban-account deployer`,
	Args:         cobra.ExactArgs(1),
	RunE:         runRegister,
	SilenceUsage: true,
}

// NewRegisterCommand returns the watchdog registration subcommand.
func NewRegisterCommand() *cobra.Command {
	return registerCmd
}

var (
	registerName             string
	registerInterval         time.Duration
	registerWatchdogContract string
	registerSecretKey        string
	registerSourceAccount    string
	registerSorobanAccount   string
	registerNetwork          string
	registerRPCURL           string
	registerPassphrase       string
	registerSorobanBin       string
	registerDryRun           bool
	registerSimulate         bool
)

func init() {
	f := registerCmd.Flags()
	f.StringVar(&registerName, "name", "", "Registration name stored on-chain as a Soroban Symbol (letters, digits, underscores; max 9 chars)")
	f.DurationVar(&registerInterval, "interval", 0, "Expected health-check interval (e.g. 300s, 5m, 1h; must be > 0)")
	f.StringVar(&registerWatchdogContract, "watchdog-contract-id", "", "Watchdog contract id (C...); defaults to $SOROLENS_WATCHDOG_CONTRACT_ID, then $WATCHDOG_CONTRACT_ID")
	f.StringVar(&registerSecretKey, "secret-key", "", "Stellar secret seed (S...); falls back to $SOROLENS_WATCHDOG_SECRET")
	f.StringVar(&registerSourceAccount, "source-account", "", "Stellar secret seed or account address used to sign (alias of --secret-key)")
	f.StringVar(&registerSorobanAccount, "soroban-account", "", "Named source account already configured in the soroban CLI (soroban keys add)")
	f.StringVar(&registerNetwork, "network", "testnet", "Stellar network (testnet, mainnet, futurenet)")
	f.StringVar(&registerRPCURL, "rpc-url", defaultRPCURL, "Soroban RPC endpoint")
	f.StringVar(&registerPassphrase, "network-passphrase", defaultPassphrase, "Network passphrase")
	f.StringVar(&registerSorobanBin, "soroban-bin", "soroban", "Path to the soroban CLI binary")
	f.BoolVar(&registerDryRun, "dry-run", false, "Print the soroban CLI invocation without executing it")
	f.BoolVar(&registerSimulate, "simulate", false, "Simulate the invocation instead of submitting it (no transaction is applied)")
}

// runRegister executes the register subcommand.
func runRegister(cmd *cobra.Command, args []string) error {
	monitored := args[0]

	// Resolve credentials: explicit flags win, then the environment, then a
	// named soroban CLI account.
	secret := registerSecretKey
	if secret == "" {
		secret = registerSourceAccount
	}
	if secret == "" {
		secret = os.Getenv("SOROLENS_WATCHDOG_SECRET")
	}
	watchdogContract := registerWatchdogContract
	if watchdogContract == "" {
		watchdogContract = os.Getenv("SOROLENS_WATCHDOG_CONTRACT_ID")
	}
	if watchdogContract == "" {
		watchdogContract = os.Getenv("WATCHDOG_CONTRACT_ID")
	}

	opts := registerOptions{
		MonitoredContractID: monitored,
		Name:                registerName,
		Interval:            registerInterval,
		WatchdogContractID:  watchdogContract,
		SecretKey:           secret,
		SorobanAccount:      registerSorobanAccount,
		Network:             registerNetwork,
		RPCURL:              registerRPCURL,
		Passphrase:          registerPassphrase,
		SorobanBin:          registerSorobanBin,
		Simulate:            registerSimulate,
	}

	inv, err := buildRegisterInvocation(opts)
	if err != nil {
		return err
	}

	jsonOut, _ := cmd.Flags().GetBool("json")

	if registerDryRun {
		if jsonOut {
			return format.PrintJSON(map[string]any{
				"dry_run":     true,
				"command":     inv.Args,
				"network":     inv.Network,
				"watchdog_id": inv.WatchdogContractID,
			})
		}
		fmt.Println("dry-run: would execute:")
		fmt.Println(shellQuoteArgs(inv.Args))
		return nil
	}

	if jsonOut {
		out, err := execRegister(cmd.Context(), inv)
		if err != nil {
			return err
		}
		return format.PrintJSON(out)
	}

	fmt.Fprintf(os.Stderr, "Invoking %s on %s ...\n", registerFuncName, inv.WatchdogContractID)
	out, err := execRegister(cmd.Context(), inv)
	if err != nil {
		return err
	}
	fmt.Println("Registered", inv.Name, "->", inv.MonitoredContractID)
	fmt.Println("Watchdog contract:", inv.WatchdogContractID)
	fmt.Println("Network:", inv.Network)
	if out.TxHash != "" {
		fmt.Println("Transaction:", out.TxHash)
	}
	if out.ResultXDR != "" {
		fmt.Println("Result XDR:", out.ResultXDR)
	}
	return nil
}

// registerOptions carries every input the invocation builder needs, so tests
// can exercise validation without touching flag globals.
type registerOptions struct {
	MonitoredContractID string
	Name                string
	Interval            time.Duration
	WatchdogContractID  string
	SecretKey           string
	SorobanAccount      string
	Network             string
	RPCURL              string
	Passphrase          string
	SorobanBin          string
	Simulate            bool
}

// invocation describes a fully-resolved soroban CLI invocation.
type invocation struct {
	Args                []string
	WatchdogContractID  string
	MonitoredContractID string
	Name                string
	Network             string
}

// registerInvocationResult is the parsed stdout of a successful invocation.
type registerInvocationResult struct {
	TxHash    string `json:"tx_hash,omitempty"`
	ResultXDR string `json:"result_xdr,omitempty"`
}

// namePattern matches the Soroban short-symbol character set accepted by the
// watchdog contract's Symbol name field.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9_]{1,9}$`)

// buildRegisterInvocation validates all inputs and assembles the soroban CLI
// argument vector that performs the on-chain register_contract call. It has
// no side effects, which makes --dry-run a pure projection of these inputs.
func buildRegisterInvocation(opts registerOptions) (*invocation, error) {
	if err := validateContractID(opts.MonitoredContractID); err != nil {
		return nil, err
	}
	if err := validateContractID(opts.WatchdogContractID); err != nil {
		return nil, err
	}
	if !namePattern.MatchString(opts.Name) {
		return nil, fmt.Errorf("invalid --name %q: must be 1-9 characters of letters, digits or underscores (Soroban Symbol)", opts.Name)
	}
	if opts.Interval <= 0 {
		return nil, errors.New("--interval must be greater than zero (e.g. --interval 300s)")
	}
	intervalSeconds := uint64(opts.Interval / time.Second)
	if intervalSeconds == 0 {
		return nil, errors.New("--interval must be at least one second")
	}

	// Exactly one credential source must be resolvable. A raw secret seed is
	// validated (strkey shape + checksum); a named account is assumed valid
	// and left to the soroban CLI to resolve.
	switch {
	case opts.SecretKey != "":
		if err := validateSource(opts.SecretKey); err != nil {
			return nil, err
		}
	case opts.SorobanAccount != "":
		// fine: delegated to the soroban CLI
	default:
		return nil, errors.New("no credentials: pass --secret-key/--source-account, set SOROLENS_WATCHDOG_SECRET, or pass --soroban-account")
	}

	args := []string{
		opts.SorobanBin,
		"contract", "invoke",
		"--id", opts.WatchdogContractID,
		"--network", opts.Network,
		"--rpc-url", opts.RPCURL,
		"--network-passphrase", opts.Passphrase,
	}
	if opts.SecretKey != "" {
		args = append(args, "--source-account", opts.SecretKey)
	} else {
		args = append(args, "--source-account", opts.SorobanAccount)
	}
	if opts.Simulate {
		// --send=no asks the soroban CLI to simulate the call without
		// broadcasting a transaction.
		args = append(args, "--send=no")
	}
	// "--" terminates soroban CLI flags; everything after it is the contract
	// function and its arguments.
	args = append(args,
		"--",
		registerFuncName,
		"--caller", deriveCallerArg(opts.SecretKey),
		"--contract_id", opts.MonitoredContractID,
		"--name", opts.Name,
		"--check_interval", strconv.FormatUint(intervalSeconds, 10),
	)

	return &invocation{
		Args:                args,
		WatchdogContractID:  opts.WatchdogContractID,
		MonitoredContractID: opts.MonitoredContractID,
		Name:                opts.Name,
		Network:             opts.Network,
	}, nil
}

// deriveCallerArg resolves the value for the contract's caller parameter.
// The contract expects an Address, so a secret seed is converted to its
// public 'G...' address; a named soroban account is passed through as-is
// (the soroban CLI resolves named accounts for address arguments).
func deriveCallerArg(secret string) string {
	if secret == "" {
		return ""
	}
	if addr, err := deriveAccountAddress(secret); err == nil {
		return addr
	}
	return secret
}

// validateSource accepts either a secret seed ('S...') or a public account
// address ('G...'). Public addresses cannot sign but are permitted so the
// caller can preview or simulate an invocation whose signature is supplied
// elsewhere.
func validateSource(s string) error {
	if strings.HasPrefix(s, "G") {
		return validateAccountID(s)
	}
	return validateSeed(s)
}

// runSoroban is the process runner, a package variable so tests can capture
// the invocation without executing the soroban binary.
var runSoroban = func(ctx context.Context, bin string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		prefix := strings.Join(args[:min(2, len(args))], " ")
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s %s failed: %s: %w", bin, prefix, msg, err)
		}
		return nil, fmt.Errorf("%s %s failed: %w", bin, prefix, err)
	}
	return out, nil
}

// execRegister runs the invocation and parses the transaction result.
func execRegister(ctx context.Context, inv *invocation) (*registerInvocationResult, error) {
	// --output json must come before the "--" separator (it belongs to the
	// soroban CLI, not the contract call).
	cliArgs := append([]string{}, inv.Args...)
	sep := -1
	for i, a := range cliArgs {
		if a == "--" {
			sep = i
			break
		}
	}
	withJSON := append([]string{}, cliArgs[:sep]...)
	withJSON = append(withJSON, "--output", "json")
	withJSON = append(withJSON, cliArgs[sep:]...)

	out, err := runSoroban(ctx, cliArgs[0], withJSON)
	if err != nil {
		return nil, err
	}
	var res registerInvocationResult
	if err := json.Unmarshal(out, &res); err != nil {
		// The CLI printed something other than the expected JSON (e.g.
		// human-readable output); treat the call as successful without a
		// parsable result rather than failing the command.
		return &registerInvocationResult{}, nil
	}
	return &res, nil
}

// shellQuoteArgs renders an argument vector as a shell-quotable line, used
// by --dry-run so the output can be copy-pasted.
func shellQuoteArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		switch {
		case a == "":
			quoted[i] = "''"
		case strings.ContainsAny(a, " \t\n\"'\\$"):
			quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		default:
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

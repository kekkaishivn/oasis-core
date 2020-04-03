// Oasis network integration test harness.
package main

import (
	"fmt"

	"github.com/oasislabs/oasis-core/go/oasis-test-runner/cmd"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario/e2e"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario/remotesigner"
)

func main() {
	// The general idea is that it should be possible to reuse everything
	// except for the main() function  and specialized test cases to write
	// test drivers that need to exercise the Oasis network or things built
	// up on the Oasis network.
	//
	// Other implementations will likely want to override parts of rootCmd,
	// in particular the `Use`, `Short`, and `Version` fields.
	rootCmd := cmd.RootCmd()

	// Register the e2e test cases.
	rootCmd.Flags().AddFlagSet(e2e.Flags)
	for _, s := range []scenario.Scenario{
		// Basic test.
		e2e.Basic,
		e2e.BasicEncryption,
		// Byzantine executor node.
		e2e.ByzantineExecutorHonest,
		e2e.ByzantineExecutorWrong,
		e2e.ByzantineExecutorStraggler,
		// Byzantine merge node.
		e2e.ByzantineMergeHonest,
		e2e.ByzantineMergeWrong,
		e2e.ByzantineMergeStraggler,
		// Storage sync test.
		e2e.StorageSync,
		// Sentry test.
		e2e.Sentry,
		e2e.SentryEncryption,
		// Keymanager restart test.
		e2e.KeymanagerRestart,
		// Dump/restore test.
		e2e.DumpRestore,
		// Halt test.
		e2e.HaltRestore,
		// Multiple runtimes test.
		e2e.MultipleRuntimes,
		// Registry CLI test.
		e2e.RegistryCLI,
		// Stake CLI test.
		e2e.StakeCLI,
		// Node shutdown test.
		e2e.NodeShutdown,
		// Gas fees tests.
		e2e.GasFeesStaking,
		e2e.GasFeesStakingDumpRestore,
		e2e.GasFeesRuntimes,
		// Identity CLI test.
		e2e.IdentityCLI,
		// Runtime prune test.
		e2e.RuntimePrune,
		// Runtime dynamic registration test.
		e2e.RuntimeDynamic,
		// Transaction source test.
		e2e.TxSourceMultiShort,
		e2e.TxSourceMulti,
		// Node upgrade tests.
		e2e.NodeUpgrade,
		e2e.NodeUpgradeCancel,
		// Debonding entries from genesis test.
		e2e.Debond,
		// Register the remote signer test cases.
		remotesigner.Basic,
	} {
		if err := cmd.Register(s); err != nil {
			fmt.Println(err)
			return
		}
	}

	// Execute the command, now that everything has been initialized.
	cmd.Execute()
}

// Oasis network integration test harness.
package main

import (
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/cmd"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario/e2e"
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
	_ = cmd.RegisterDefaultScenarios()

	// Execute the command, now that everything has been initialized.
	cmd.Execute()
}

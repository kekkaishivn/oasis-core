// Package cmd implements ba cmd tool.
package cmd

import (
	"fmt"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"

	"github.com/oasislabs/oasis-core/go/common/version"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/metrics"
)

var (
	rootCmd = &cobra.Command{
		Use:     "ba",
		Short:   "Benchmark analysis",
		Version: version.SoftwareVersion,
	}

	rootFlags = flag.NewFlagSet("", flag.ContinueOnError)
)

// RootCommand returns the root (top level) cobra.Command.
func RootCommand() *cobra.Command {
	return rootCmd
}

// Execute spawns the main entry point after handling the command line arguments.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		common.EarlyLogAndExit(err)
	}
}

func init() {
	rootFlags.String(metrics.CfgMetricsAddr, "http://localhost:9090", "Prometheus query address")
	_ = viper.BindPFlags(rootFlags)
	rootCmd.PersistentFlags().AddFlagSet(rootFlags)

	if err := logging.Initialize(os.Stdout, logging.FmtJSON, logging.LevelInfo, nil); err != nil {
		fmt.Println(fmt.Errorf("root: failed to initialize logging: %w", err))
	}
	// Register all of the sub-commands.
	RegisterBaCmd(rootCmd)
}

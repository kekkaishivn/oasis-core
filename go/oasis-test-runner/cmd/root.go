// Package cmd implements the commands for the test-runner executable.
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/version"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
	cmdFlags "github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/flags"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/metrics"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/env"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario/e2e"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario/remotesigner"
)

const (
	cfgConfigFile       = "config"
	cfgLogFmt           = "log.format"
	cfgLogLevel         = "log.level"
	cfgLogNoStdout      = "log.no_stdout"
	cfgNumRuns          = "num_runs"
	CfgTest             = "test"
	CfgTestP            = "t"
	cfgParallelJobCount = "parallel.job_count"
	cfgParallelJobIndex = "parallel.job_index"
)

var (
	rootCmd = &cobra.Command{
		Use:     "oasis-test-runner",
		Short:   "Oasis Test Runner",
		Version: version.SoftwareVersion,
		RunE:    runRoot,
	}

	listCmd = &cobra.Command{
		Use:   "list",
		Short: "List registered test cases",
		Run:   runList,
	}

	rootFlags       = flag.NewFlagSet("", flag.ContinueOnError)
	TestParamsFlags = flag.NewFlagSet("", flag.ContinueOnError)
	TestParamsMask  = "params.%s.%s"

	cfgFile string
	numRuns int

	Scenarios        = make(map[string]scenario.Scenario)
	defaultScenarios []scenario.Scenario

	// oasis-test-runner-specific metrics.
	upGauge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "up",
			Help: "Is oasis-test-runner test active",
		},
	)

	oasisTestRunnerCollectors = []prometheus.Collector{
		upGauge,
	}

	pusher              *push.Pusher
	oasisTestRunnerOnce sync.Once
)

// RootCmd returns the root command's structure that will be executed, so that
// it can be used to alter the configuration and flags of the command.
//
// Note: `Run` is pre-initialized to the main entry point of the test harness,
// and should likely be left un-altered.
func RootCmd() *cobra.Command {
	return rootCmd
}

// Execute spawns the main entry point after handing the config file.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		common.EarlyLogAndExit(err)
	}
}

// RegisterNondefault adds a scenario to the runner.
func RegisterNondefault(s scenario.Scenario) error {
	n := strings.ToLower(s.Name())
	if _, ok := Scenarios[n]; ok {
		return fmt.Errorf("root: scenario already registered: %s", n)
	}

	Scenarios[n] = s

	params := s.Parameters()
	if len(params) > 0 {
		for k, v := range scenario.ParametersToStringMap(params) {
			// Populate TestParamsFlags with test parameters and (re-)register it.
			param := fmt.Sprintf(TestParamsMask, n, k)
			TestParamsFlags.StringSlice(param, []string{v}, fmt.Sprintf("value(s) of parameter %s for test %s", k, n))
			rootCmd.PersistentFlags().AddFlagSet(TestParamsFlags)
			_ = viper.BindPFlag(param, TestParamsFlags.Lookup(param))
		}
	}

	return nil
}

// parseTestParams parses --params.<test_name>.<key1>=<val1>,<val2>... flags combinations, clones provided proto-
// scenarios, and populates them so that each scenario instance has unique paramater set. Returns mapping test name ->
// list of scenario instances.
func parseTestParams(toRun []scenario.Scenario) (map[string][]scenario.Scenario, error) {
	r := make(map[string][]scenario.Scenario)
	for _, s := range toRun {
		zippedParams := make(map[string][]string)
		for k := range s.Parameters() {
			userVal := viper.GetStringSlice("params." + s.Name() + "." + k)
			if userVal == nil {
				continue
			}
			zippedParams[k] = userVal
		}

		parameterSets := computeParamSets(zippedParams, map[string]string{})

		// For each parameter set combination, clone a scenario and apply user-provided parameter value.
		for _, ps := range parameterSets {
			sCloned := s.Clone()
			for k, userVal := range ps {
				v := sCloned.Parameters()[k]
				switch v := v.(type) {
				case *int:
					val, err := strconv.ParseInt(userVal, 10, 32)
					if err != nil {
						return nil, err
					}
					*v = int(val)
				case *int64:
					val, err := strconv.ParseInt(userVal, 10, 64)
					if err != nil {
						return nil, err
					}
					*v = val
				case *float64:
					val, err := strconv.ParseFloat(userVal, 64)
					if err != nil {
						return nil, err
					}
					*v = val
				case *bool:
					val, err := strconv.ParseBool(userVal)
					if err != nil {
						return nil, err
					}
					*v = val
				case *string:
					*v = userVal
				default:
					return nil, fmt.Errorf("cannot parse parameter. Unknown type %T of value %v", v, v)
				}
			}
			r[s.Name()] = append(r[s.Name()], sCloned)
		}

		// No parameters provided over CLI, keep a single copy.
		if len(parameterSets) == 0 {
			r[s.Name()] = []scenario.Scenario{s}
		}
	}

	return r, nil
}

// computeParamSets recursively combines a map of string slices into all possible key=>value parameter sets.
func computeParamSets(zp map[string][]string, ps map[string]string) []map[string]string {
	// Recursion stops when zp is empty. Append ps to result set.
	if len(zp) == 0 {
		if len(ps) == 0 {
			return []map[string]string{}
		}

		psCloned := map[string]string{}
		for k, v := range ps {
			psCloned[k] = v
		}
		return []map[string]string{psCloned}
	}

	rps := []map[string]string{}

	// Take first element from cloned zp and do recursion deterministically.
	var zpKeys []string
	for k := range zp {
		zpKeys = append(zpKeys, k)
	}
	sort.Strings(zpKeys)

	zpCloned := map[string][]string{}
	for _, k := range zpKeys[1:] {
		zpCloned[k] = zp[k]
	}
	for _, v := range zp[zpKeys[0]] {
		ps[zpKeys[0]] = v
		rps = append(rps, computeParamSets(zpCloned, ps)...)
	}

	return rps
}

// RegisterDefaultScenarios registers all available default scenarios and returns any errors encountered.
func RegisterDefaultScenarios() (ers []error) {
	var errors []error
	// Basic test.
	errors = append(errors, Register(e2e.Basic))
	errors = append(errors, Register(e2e.BasicEncryption))
	// Byzantine executor node.
	errors = append(errors, Register(e2e.ByzantineExecutorHonest))
	errors = append(errors, Register(e2e.ByzantineExecutorWrong))
	errors = append(errors, Register(e2e.ByzantineExecutorStraggler))
	// Byzantine merge node.
	errors = append(errors, Register(e2e.ByzantineMergeHonest))
	errors = append(errors, Register(e2e.ByzantineMergeWrong))
	errors = append(errors, Register(e2e.ByzantineMergeStraggler))
	// Storage sync test.
	errors = append(errors, Register(e2e.StorageSync))
	// Sentry test.
	errors = append(errors, Register(e2e.Sentry))
	errors = append(errors, Register(e2e.SentryEncryption))
	// Keymanager restart test.
	errors = append(errors, Register(e2e.KeymanagerRestart))
	// Dump/restore test.
	errors = append(errors, Register(e2e.DumpRestore))
	// Halt test.
	errors = append(errors, Register(e2e.HaltRestore))
	// Multiple runtimes test.
	errors = append(errors, Register(e2e.MultipleRuntimes))
	// Registry CLI test.
	errors = append(errors, Register(e2e.RegistryCLI))
	// Stake CLI test.
	errors = append(errors, Register(e2e.StakeCLI))
	// Node shutdown test.
	errors = append(errors, Register(e2e.NodeShutdown))
	// Gas fees tests.
	errors = append(errors, Register(e2e.GasFeesStaking))
	errors = append(errors, Register(e2e.GasFeesStakingDumpRestore))
	errors = append(errors, Register(e2e.GasFeesRuntimes))
	// Identity CLI test.
	errors = append(errors, Register(e2e.IdentityCLI))
	// Runtime prune test.
	errors = append(errors, Register(e2e.RuntimePrune))
	// Runtime dynamic registration test.
	errors = append(errors, Register(e2e.RuntimeDynamic))
	// Transaction source test.
	errors = append(errors, Register(e2e.TxSourceMultiShort))
	errors = append(errors, RegisterNondefault(e2e.TxSourceMulti))
	// Node upgrade tests.
	errors = append(errors, Register(e2e.NodeUpgrade))
	errors = append(errors, Register(e2e.NodeUpgradeCancel))
	// Debonding entries from genesis test.
	errors = append(errors, Register(e2e.Debond))

	// Register the remote signer test cases.
	errors = append(errors, Register(remotesigner.Basic))

	// Filter out nil errors.
	for _, e := range errors {
		if e != nil {
			ers = append(ers, e)
		}
	}

	return
}

// Register adds a scenario to the runner and the default scenarios list.
func Register(scenario scenario.Scenario) error {
	if err := RegisterNondefault(scenario); err != nil {
		return err
	}

	defaultScenarios = append(defaultScenarios, scenario)
	return nil
}

func initRootEnv(cmd *cobra.Command) (*env.Env, error) {
	// Initialize the root dir.
	rootDir := env.GetRootDir()
	if err := rootDir.Init(cmd); err != nil {
		return nil, err
	}
	env := env.New(rootDir)

	var ok bool
	defer func() {
		if !ok {
			env.Cleanup()
		}
	}()

	var logFmt logging.Format
	if err := logFmt.Set(viper.GetString(cfgLogFmt)); err != nil {
		return nil, fmt.Errorf("root: failed to set log format: %v", err)
	}

	var logLevel logging.Level
	if err := logLevel.Set(viper.GetString(cfgLogLevel)); err != nil {
		return nil, fmt.Errorf("root: failed to set log level: %v", err)
	}

	// Initialize logging.
	logFile := filepath.Join(env.Dir(), "test-runner.log")
	w, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("root: failed to open log file: %v", err)
	}

	var logWriter io.Writer = w
	if !viper.GetBool(cfgLogNoStdout) {
		logWriter = io.MultiWriter(os.Stdout, w)
	}
	if err := logging.Initialize(logWriter, logFmt, logLevel, nil); err != nil {
		return nil, fmt.Errorf("root: failed to initialize logging: %v", err)
	}

	ok = true
	return env, nil
}

func runRoot(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	if viper.GetString(metrics.CfgMetricsAddr) != "" {
		oasisTestRunnerOnce.Do(func() {
			prometheus.MustRegister(oasisTestRunnerCollectors...)
		})
	}

	// Initialize the base dir, logging, etc.
	rootEnv, err := initRootEnv(cmd)
	if err != nil {
		return err
	}
	defer rootEnv.Cleanup()
	logger := logging.GetLogger("test-runner")

	// Enumerate the requested test cases.
	toRun := defaultScenarios // Run all default scenarios if not set.
	if vec := viper.GetStringSlice(CfgTest); len(vec) > 0 {
		toRun = nil
		for _, v := range vec {
			n := strings.ToLower(v)
			scenario, ok := Scenarios[v]
			if !ok {
				logger.Error("unknown test case",
					"test", n,
				)
				return fmt.Errorf("root: unknown test case: %s", n)
			}
			toRun = append(toRun, scenario)
		}
	}

	excludeMap := make(map[string]bool)
	if excludeEnv := os.Getenv("OASIS_EXCLUDE_E2E"); excludeEnv != "" {
		for _, v := range strings.Split(excludeEnv, ",") {
			excludeMap[strings.ToLower(v)] = true
		}
	}

	// Run the required test scenarios.
	parallelJobCount := viper.GetInt(cfgParallelJobCount)
	parallelJobIndex := viper.GetInt(cfgParallelJobIndex)

	// Parse test parameters passed by CLI.
	var toRunExploded map[string][]scenario.Scenario
	toRunExploded, err = parseTestParams(toRun)
	if err != nil {
		return fmt.Errorf("root: failed to parse test params: %v", err)
	}

	// Run all test instances.
	index := 0
	for run := 0; run < numRuns; run++ {
		for name, sc := range toRunExploded {
			for i, v := range sc {
				// Maintain unique scenario datadir.
				runID := run*len(sc) + i
				n := fmt.Sprintf("%s/%d", name, runID)

				if index%parallelJobCount != parallelJobIndex {
					logger.Info("skipping test case (assigned to different parallel job)",
						"test", name, "run_id", runID,
					)
					index++
					continue
				}

				if excludeMap[strings.ToLower(v.Name())] {
					logger.Info("skipping test case (excluded by environment)",
						"test", name, "run_id", runID,
					)
					index++
					continue
				}

				logger.Info("running test case",
					"test", name, "run_id", runID,
				)

				childEnv, err := rootEnv.NewChild(n, env.TestInstanceInfo{
					Test:         v.Name(),
					Instance:     filepath.Base(rootEnv.Dir()),
					ParameterSet: scenario.ParametersToStringMap(v.Parameters()),
					Run:          run,
				})
				if err != nil {
					logger.Error("failed to setup child environment",
						"err", err,
						"test", name,
						"run_id", runID,
					)
					return fmt.Errorf("root: failed to setup child environment: %v", err)
				}

				// Dump current parameter set to file.
				if err = childEnv.WriteTestInstanceInfo(); err != nil {
					return err
				}

				// Init per-run prometheus pusher, if metrics are enabled.
				if viper.GetString(metrics.CfgMetricsAddr) != "" {
					pusher = push.New(viper.GetString(metrics.CfgMetricsAddr), oasis.MetricsJobName)
					pusher = pusher.
						Grouping(oasis.MetricsLabelInstance, childEnv.TestInfo().Instance).
						Grouping(oasis.MetricsLabelRun, strconv.Itoa(childEnv.TestInfo().Run)).
						Grouping(oasis.MetricsLabelTest, childEnv.TestInfo().Test).
						Grouping(oasis.MetricsLabelSoftwareVersion, version.SoftwareVersion).
						Grouping(oasis.MetricsLabelGitBranch, version.GitBranch)

					// Populate test-provided parameters.
					for k, v := range childEnv.TestInfo().ParameterSet {
						pusher = pusher.Grouping(metrics.EscapeLabelCharacters(k), v)
					}

					pusher = pusher.Gatherer(prometheus.DefaultGatherer)
				}

				if err = doScenario(childEnv, v); err != nil {
					logger.Error("failed to run test case",
						"err", err,
						"test", name,
						"run_id", runID,
					)
					err = fmt.Errorf("root: failed to run test case: %v", err)
				}

				if cleanErr := doCleanup(childEnv); cleanErr != nil {
					logger.Error("failed to clean up child envionment",
						"err", cleanErr,
						"test", name,
						"run_id", runID,
					)
					if err == nil {
						err = fmt.Errorf("root: failed to clean up child enviroment: %v", cleanErr)
					}
				}

				if err != nil {
					return err
				}

				logger.Info("passed test case",
					"test", name, "run_id", runID,
				)

				index++
			}
		}
	}

	return nil
}

func doScenario(childEnv *env.Env, sc scenario.Scenario) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("root: panic caught running test case: %v", r)
		}
	}()

	var fixture *oasis.NetworkFixture
	if fixture, err = sc.Fixture(); err != nil {
		err = fmt.Errorf("root: failed to initialize network fixture: %v", err)
		return
	}

	// Instantiate fixture if it is non-nil. Otherwise assume Init will do
	// something on its own.
	var net *oasis.Network
	if fixture != nil {
		if net, err = fixture.Create(childEnv); err != nil {
			err = fmt.Errorf("root: failed to instantiate fixture: %v", err)
			return
		}
	}

	if err = sc.Init(childEnv, net); err != nil {
		err = fmt.Errorf("root: failed to initialize test case: %v", err)
		return
	}

	if pusher != nil {
		upGauge.Set(1.0)
		if err = pusher.Push(); err != nil {
			err = fmt.Errorf("root: failed to push metrics: %v", err)
			return
		}
	}

	if err = sc.Run(childEnv); err != nil {
		err = fmt.Errorf("root: failed to run test case: %v", err)
		return
	}

	if pusher != nil {
		upGauge.Set(0.0)
		if err = pusher.Push(); err != nil {
			err = fmt.Errorf("root: failed to push metrics: %v", err)
			return
		}
	}

	return
}

func doCleanup(childEnv *env.Env) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("root: panic caught cleaning up test case: %v", r)
		}
	}()

	childEnv.Cleanup()

	return
}

func runList(cmd *cobra.Command, args []string) {
	switch len(Scenarios) {
	case 0:
		fmt.Printf("No supported test cases!\n")
	default:
		fmt.Printf("Supported test cases:\n")

		// Sort scenarios alphabetically before printing.
		var scenarioNames []string
		for name := range Scenarios {
			scenarioNames = append(scenarioNames, name)
		}
		sort.Strings(scenarioNames)

		for _, n := range scenarioNames {
			fmt.Printf("  * %v", n)
			params := Scenarios[n].Parameters()
			if len(params) > 0 {
				fmt.Printf(" (parameters:")
				for p := range params {
					fmt.Printf(" %v", p)
				}
				fmt.Printf(")")
			}
			fmt.Printf("\n")
		}
	}
}

func init() {
	logFmt := logging.FmtLogfmt
	logLevel := logging.LevelWarn

	// Register flags.
	rootFlags.StringVar(&cfgFile, cfgConfigFile, "", "config file")
	rootFlags.Var(&logFmt, cfgLogFmt, "log format")
	rootFlags.Var(&logLevel, cfgLogLevel, "log level")
	rootFlags.Bool(cfgLogNoStdout, false, "do not mutiplex logs to stdout")
	rootFlags.StringSliceP(CfgTest, CfgTestP, nil, "test(s) to run")
	rootFlags.String(metrics.CfgMetricsAddr, "", "metrics (prometheus) pushgateway address")
	rootFlags.Duration(metrics.CfgMetricsPushInterval, 5*time.Second, "push interval for node exporter and oasis nodes")
	rootFlags.IntVarP(&numRuns, cfgNumRuns, "n", 1, "number of runs for given test(s)")
	rootFlags.Int(cfgParallelJobCount, 1, "(for CI) number of overall parallel jobs")
	rootFlags.Int(cfgParallelJobIndex, 0, "(for CI) index of this parallel job")
	_ = viper.BindPFlags(rootFlags)

	rootCmd.PersistentFlags().AddFlagSet(rootFlags)
	rootCmd.PersistentFlags().AddFlagSet(env.Flags)
	rootCmd.AddCommand(listCmd)

	cobra.OnInitialize(func() {
		if cfgFile != "" {
			viper.SetConfigFile(cfgFile)
			if err := viper.ReadInConfig(); err != nil {
				common.EarlyLogAndExit(err)
			}
		}

		viper.Set(cmdFlags.CfgDebugDontBlameOasis, true)
	})
}

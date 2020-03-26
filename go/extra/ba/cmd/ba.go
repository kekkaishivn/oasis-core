package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/api"
	"github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/metrics"
	testCmd "github.com/oasislabs/oasis-core/go/oasis-test-runner/cmd"
	testOasis "github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis"
)

const (
	cfgMetrics                = "metrics"
	cfgMetricsP               = "m"
	cfgMetricsTargetGitBranch = "metrics.target.git_branch"
	cfgMetricsSourceGitBranch = "metrics.source.git_branch"
)

var (
	cmpFlags = flag.NewFlagSet("", flag.ContinueOnError)

	cmpCmd = &cobra.Command{
		Use:   "cmp",
		Short: "compare last two benchmark instances",
		Long: `cmp connects to prometheus, fetches the metrics of the benchmark instances and
compares them. By default, the most recent instance (source) is fetched and
compared to the pre-last (target). If --metrics.{target|source}.git_branch is
provided, it compares the most recent instances in the corresponding branches.
cmp compares all metrics provided by -m parameter and computes ratio source/target
of metric values. If any of the metrics exceeds max_threshold.<metric>.{avg|max}_ratio
or doesn't reach min_threshold.<metric>.{avg|max}_ratio, ba exits with error code 1.`,
		Run: runCmp,
	}

	allMetrics = map[string]*Metric{
		"time": &Metric{
			getter:               getDuration,
			maxThresholdAvgRatio: 1.1,
			maxThresholdMaxRatio: 1.1,
		},
		"du": &Metric{
			getter:               getDiskUsage,
			maxThresholdAvgRatio: 1.06,
			maxThresholdMaxRatio: 1.15,
		},
		"io": &Metric{
			getter:               getIOWork,
			maxThresholdAvgRatio: 1.2,
			maxThresholdMaxRatio: 1.2,
		},
		"mem": &Metric{
			getter:               getRssAnonMemory,
			maxThresholdAvgRatio: 1.1,
			maxThresholdMaxRatio: 1.1,
		},
		"cpu": &Metric{
			getter:               getCPUTime,
			maxThresholdAvgRatio: 1.05,
			maxThresholdMaxRatio: 1.05,
		},
		"net": &Metric{
			getter: getNetwork,
			// Network stats suffer effects from other processes too and varies.
			maxThresholdAvgRatio: 1.3,
			maxThresholdMaxRatio: 1.3,
		},
	}
	userMetrics []string

	logger = logging.GetLogger("cmd/ba")
	client api.Client
)

type Metric struct {
	getter               func(context.Context, string, *model.SampleStream) (float64, float64, error)
	maxThresholdAvgRatio float64
	maxThresholdMaxRatio float64
	minThresholdAvgRatio float64
	minThresholdMaxRatio float64
}

// getDuration returns average and maximum running times of the given coarse benchmark instance ("up" metric w/ minute
// resolution time series).
func getDuration(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	instance := string(bi.Metric[testOasis.MetricsLabelInstance])

	// Re-fetch the given benchmark instance with second resolution. Each obtained time series corresponds to one run.
	v1api := v1.NewAPI(client)
	r := v1.Range{
		Start: bi.Values[0].Timestamp.Time().Add(-1 * time.Minute),
		End:   bi.Values[len(bi.Values)-1].Timestamp.Time().Add(time.Minute),
		Step:  time.Second,
	}

	query := fmt.Sprintf("up %s == 1.0", bi.Metric.String())
	result, warnings, err := v1api.QueryRange(ctx, query, r)
	if err != nil {
		common.EarlyLogAndExit(errors.Wrap(err, "error querying Prometheus"))
	}
	if len(warnings) > 0 {
		logger.Warn("warnings while querying Prometheus", "warnings", warnings)
	}
	if len(result.(model.Matrix)) == 0 {
		return 0, 0, fmt.Errorf("getDuration: no time series matched test: %s and instance: %s", test, instance)
	}
	// Compute average and max duration of runs. Since we have a second-resolution, each point denotes 1 second of run's
	// uptime. Just count all points and divide them by the number of runs.
	avgDuration := 0.0
	maxDuration := 0.0
	for _, s := range result.(model.Matrix) {
		avgDuration += float64(len(s.Values))
		if maxDuration < float64(len(s.Values)) {
			maxDuration = float64(len(s.Values))
		}
	}
	avgDuration /= float64(len(result.(model.Matrix)))

	return avgDuration, maxDuration, nil
}

// getIOWork returns average and maximum sum of read and written bytes by all workers of the given coarse benchmark
// instance  ("up" metric).
func getIOWork(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	readAvg, readMax, err := getSummableMetric(ctx, "oasis_worker_disk_read_bytes", test, bi)
	if err != nil {
		return 0, 0, err
	}
	writtenAvg, writtenMax, err := getSummableMetric(ctx, "oasis_worker_disk_written_bytes", test, bi)
	if err != nil {
		return 0, 0, err
	}

	return readAvg + writtenAvg, readMax + writtenMax, nil
}

// getDiskUsage returns average and maximum sum of disk usage for all workers of the given coarse benchmark instance
// ("up" metric).
func getDiskUsage(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	return getSummableMetric(ctx, "oasis_worker_disk_usage_bytes", test, bi)
}

// getRssAnonMemory returns average and maximum sum of anonymous resident memory for all workers of the given coarse
// benchmark instance ("up" metric).
func getRssAnonMemory(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	return getSummableMetric(ctx, "oasis_worker_mem_RssAnon_bytes", test, bi)
}

// getCPUTime returns average and maximum sum of utime and stime for all workers of the given coarse benchmark instance
// ("up" metric).
func getCPUTime(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	utimeAvg, utimeMax, err := getSummableMetric(ctx, "oasis_worker_cpu_utime_seconds", test, bi)
	if err != nil {
		return 0, 0, err
	}
	stimeAvg, stimeMax, err := getSummableMetric(ctx, "oasis_worker_cpu_stime_seconds", test, bi)
	if err != nil {
		return 0, 0, err
	}

	return utimeAvg + stimeAvg, utimeMax + stimeMax, nil
}

// getSummableMetric returns average and maximum sum of metrics for all workers of the given coarse benchmark instance
// ("up" metric).
func getSummableMetric(ctx context.Context, metric string, test string, bi *model.SampleStream) (float64, float64, error) {
	instance := string(bi.Metric[testOasis.MetricsLabelInstance])

	labels := bi.Metric.Clone()
	// Existing job denotes the "oasis-test-runner" worker only. We want to sum disk space across all workers.
	delete(labels, "job")
	// We will average metric over all runs.
	delete(labels, "run")

	v1api := v1.NewAPI(client)

	query := fmt.Sprintf("sum by (run) (%s %s)", metric, labels.String())

	// Fetch value at last recorded time. Some metrics might not be available anymore, if prometheus was shut down.
	// Add one additional minute to capture reported values within the last minute period.
	t := bi.Values[len(bi.Values)-1].Timestamp.Time().Add(time.Minute)

	result, warnings, err := v1api.Query(ctx, query, t)
	if err != nil {
		common.EarlyLogAndExit(errors.Wrap(err, "error querying Prometheus"))
	}
	if len(warnings) > 0 {
		logger.Warn("warnings while querying Prometheus", "warnings", warnings)
	}
	if len(result.(model.Vector)) == 0 {
		return 0, 0, fmt.Errorf("getSummableMetric: no time series matched test: %s and instance: %s", test, instance)
	}

	// Compute average and max values.
	avg := 0.0
	max := 0.0
	for _, s := range result.(model.Vector) {
		avg += float64(s.Value)
		if max < float64(s.Value) {
			max = float64(s.Value)
		}
	}
	avg /= float64(len(result.(model.Vector)))

	return avg, max, nil
}

// getNetwork returns average and maximum amount of received and transmitted bytes for all workers of the given coarse
// benchmark instance ("up" metric).
func getNetwork(ctx context.Context, test string, bi *model.SampleStream) (float64, float64, error) {
	instance := string(bi.Metric[testOasis.MetricsLabelInstance])

	labels := bi.Metric.Clone()
	// We will group by job to fetch traffic across all workers.
	delete(labels, "job")
	// We will average metric over all runs.
	delete(labels, "run")

	v1api := v1.NewAPI(client)
	r := v1.Range{
		Start: bi.Values[0].Timestamp.Time().Add(-1 * time.Minute),
		End:   bi.Values[len(bi.Values)-1].Timestamp.Time().Add(time.Minute),
		Step:  time.Second,
	}

	// We store total network traffic values. Compute the difference.
	bytesTotalAvg := map[string]float64{}
	bytesTotalMax := map[string]float64{}
	for _, rxtx := range []string{"receive", "transmit"} {
		m := fmt.Sprintf("(oasis_worker_net_%s_bytes_total %s)", rxtx, labels.String())
		query := fmt.Sprintf("max by (run) %s - min by (run) %s", m, m)
		result, warnings, err := v1api.QueryRange(ctx, query, r)
		if err != nil {
			common.EarlyLogAndExit(errors.Wrap(err, "error querying Prometheus"))
		}
		if len(warnings) > 0 {
			logger.Warn("warnings while querying Prometheus", "warnings", warnings)
		}
		if len(result.(model.Matrix)) == 0 {
			return 0, 0, fmt.Errorf("getNetworkMetric: no time series matched test: %s and instance: %s", test, instance)
		}

		// Compute average and max values.
		avg := 0.0
		max := 0.0
		for _, s := range result.(model.Matrix) {
			avg += float64(s.Values[len(s.Values)-1].Value)
			if max < float64(s.Values[len(s.Values)-1].Value) {
				max = float64(s.Values[len(s.Values)-1].Value)
			}
		}
		avg /= float64(len(result.(model.Matrix)))

		bytesTotalAvg[rxtx] = avg
		bytesTotalMax[rxtx] = max
	}

	return (bytesTotalAvg["receive"] + bytesTotalAvg["transmit"]) / 2.0, (bytesTotalMax["receive"] + bytesTotalMax["transmit"]) / 2.0, nil
}

// getCoarseBenchmarkInstances finds time series based on "up" metric w/ minute resolution for the given test and gitBranch
// ordered from the oldest to the most recent ones.
//
// This function is usually called to determine test instance pairs to compare with more fine granularity and specific
// metric afterwards.
//
// NB: Due to Prometheus limit, this function fetches time series in the past 183 hours only.
func getCoarseBenchmarkInstances(ctx context.Context, test string, labels map[string]string) (model.Matrix, error) {
	v1api := v1.NewAPI(client)
	r := v1.Range{
		// XXX: Hardcoded max potential number of points in Prometheus is 11,000 which equals ~183 hours with minute resolution.
		Start: time.Now().Add(-183 * time.Hour),
		End:   time.Now(),
		Step:  time.Minute,
	}

	ls := model.LabelSet{
		"job":                      testOasis.MetricsJobName,
		testOasis.MetricsLabelTest: model.LabelValue(test),
	}
	for k, v := range labels {
		ls[model.LabelName(k)] = model.LabelValue(v)
	}

	query := fmt.Sprintf("max(up %s) by (%s) == 1.0", ls.String(), testOasis.MetricsLabelInstance)
	result, warnings, err := v1api.QueryRange(ctx, query, r)
	if err != nil {
		logger.Error("error querying Prometheus", "err", err)
		os.Exit(1)
	}
	if len(warnings) > 0 {
		logger.Warn("warnings while querying Prometheus", "warnings", warnings)
	}

	// Go through all obtained time series and order them by the timestamp of the first sample.
	sort.Slice(result.(model.Matrix), func(i, j int) bool {
		return result.(model.Matrix)[i].Values[0].Timestamp < result.(model.Matrix)[j].Values[0].Timestamp
	})
	return result.(model.Matrix), nil
}

// instanceNames extracts instance names from given Prometheus time series matrix.
func instanceNames(ts model.Matrix) []string {
	var names []string
	for _, t := range ts {
		names = append(names, instanceName(t))
	}
	return names
}

// instanceName returns the instance name label of the given sample.
func instanceName(s *model.SampleStream) string {
	return string(s.Metric[testOasis.MetricsLabelInstance])
}

// fetchAndCompare fetches the given metric from prometheus and compares the results.
//
// Returns false, if metric-specific ratios are exceeded or there is a problem obtaining time series. Otherwise true.
func fetchAndCompare(ctx context.Context, m string, test string, sInstance *model.SampleStream, tInstance *model.SampleStream) (succ bool) {
	getMetric := allMetrics[m].getter
	succ = true

	sAvg, sMax, err := getMetric(ctx, test, sInstance)
	if err != nil {
		logger.Error("error fetching source benchmark instance", "metric", m, "test", test, "instance", instanceName(sInstance), "err", err)
		return false
	}

	tAvg, tMax, err := getMetric(ctx, test, tInstance)
	if err != nil {
		logger.Error("error fetching target test instance", "metric", m, "test", test, "instance", instanceName(sInstance), "err", err)
		return false
	}

	// Compare average and max metric values and log error, if they exceed or don't reach required ratios.
	maxAvgRatio := allMetrics[m].maxThresholdAvgRatio
	maxMaxRatio := allMetrics[m].maxThresholdMaxRatio
	minAvgRatio := allMetrics[m].minThresholdAvgRatio
	minMaxRatio := allMetrics[m].minThresholdMaxRatio
	logger.Info("obtained average ratio", "metric", m, "test", test, "source_avg", sAvg, "target_avg", tAvg, "ratio", sAvg/tAvg)
	if maxAvgRatio != 0 && sAvg/tAvg > maxAvgRatio {
		logger.Error("average metric value exceeds max allowed ratio", "metric", m, "test", test, "source_avg", sAvg, "target_avg", tAvg, "ratio", sAvg/tAvg, "max_allowed_avg_ratio", maxAvgRatio)
		succ = false
	}
	if minAvgRatio != 0 && sAvg/tAvg < minAvgRatio {
		logger.Error("average metric value doesn't reach min required ratio", "metric", m, "test", test, "source_avg", sAvg, "target_avg", tAvg, "ratio", sAvg/tAvg, "min_required_avg_ratio", minAvgRatio)
		succ = false
	}
	logger.Info("obtained max ratio", "metric", m, "test", test, "source_max", sMax, "target_max", tMax, "ratio", sMax/tMax)
	if maxMaxRatio != 0 && sMax/tMax > maxMaxRatio {
		logger.Error("maximum metric value exceeds max ratio", "metric", m, "test", test, "source_max", sMax, "target_max", tMax, "ratio", sMax/tMax, "max_allowed_max_ratio", maxMaxRatio)
		succ = false
	}
	if minMaxRatio != 0 && sMax/tMax < maxMaxRatio {
		logger.Error("maximum metric value doesn't reach min required ratio", "metric", m, "test", test, "source_max", sMax, "target_max", tMax, "ratio", sMax/tMax, "min_required_max_ratio", maxMaxRatio)
		succ = false
	}

	return
}

func runCmp(cmd *cobra.Command, args []string) {
	var err error
	client, err = api.NewClient(api.Config{
		Address: viper.GetString(metrics.CfgMetricsAddr),
	})
	if err != nil {
		logger.Error("error creating client", "err", err)
		os.Exit(1)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	succ := true
	for _, test := range viper.GetStringSlice(testCmd.CfgTest) {
		labels := map[string]string{}
		if viper.IsSet(cfgMetricsSourceGitBranch) {
			labels[testOasis.MetricsLabelGitBranch] = viper.GetString(cfgMetricsSourceGitBranch)
		}

		// Query parameter value-specific tests.
		for k := range testCmd.Scenarios[test].Parameters() {
			param := fmt.Sprintf(testCmd.TestParamsMask, test, k)
			if viper.IsSet(param) {
				// TODO: Check exploded-like parameter sets like we run the tests in oasis-test-runner.
				labels[metrics.EscapeLabelCharacters(k)] = viper.GetStringSlice(param)[0]
			}
		}

		sInstances, err := getCoarseBenchmarkInstances(ctx, test, labels)
		if err != nil {
			logger.Error("error querying for source test instances", "err", err)
			os.Exit(1)
		}
		sNames := instanceNames(sInstances)
		tInstances, err := getCoarseBenchmarkInstances(ctx, test, labels)
		if err != nil {
			logger.Error("error querying for target test instances", "err", err)
			os.Exit(1)
		}
		tNames := instanceNames(tInstances)

		if len(sNames) == 0 {
			logger.Info("test does not have any source benchmark instances to compare, ignoring", "test", test)
			continue
		}
		if len(tNames) == 0 {
			logger.Info("test does not have any target benchmark instances to compare, ignoring", "test", test)
			continue
		}

		var sInstance, tInstance *model.SampleStream
		if sNames[len(sNames)-1] != tNames[len(tNames)-1] {
			// Benchmark instances differ e.g. because of different gitBranch.
			sInstance = sInstances[len(sInstances)-1]
			tInstance = tInstances[len(tInstances)-1]
		} else {
			// Last benchmark instances are equal, pick the pre-last one from the target instances.
			if len(tNames) < 2 {
				logger.Info("test has only one benchmark instance, ignoring", "test", test, "source_instances", sNames, "target_instances", tNames)
				continue
			}
			sInstance = sInstances[len(sInstances)-1]
			tInstance = tInstances[len(tInstances)-2]
		}
		logger.Info("obtained source and target instance", "test", test, "source_instance", instanceName(sInstance), "target_instance", instanceName(tInstance))

		for _, m := range userMetrics {
			// Don't put succ = succ && f oneliner here, because f won't get executed once succ = false.
			fSucc := fetchAndCompare(ctx, m, test, sInstance, tInstance)
			succ = succ && fSucc
		}
	}

	if !succ {
		os.Exit(1)
	}

	defer cancel()
}

// Register ba cmd sub-command and all of it's children.
func RegisterBaCmd(parentCmd *cobra.Command) {
	var metricNames []string
	for k := range allMetrics {
		metricNames = append(metricNames, k)
		cmpFlags.Float64Var(&allMetrics[k].maxThresholdAvgRatio, fmt.Sprintf("max_threshold.%s.avg_ratio", k), allMetrics[k].maxThresholdAvgRatio, fmt.Sprintf("maximum allowed ratio between average %s metrics", k))
		cmpFlags.Float64Var(&allMetrics[k].maxThresholdMaxRatio, fmt.Sprintf("max_threshold.%s.max_ratio", k), allMetrics[k].maxThresholdMaxRatio, fmt.Sprintf("maximum allowed ratio between maximum %s metrics", k))
		cmpFlags.Float64Var(&allMetrics[k].minThresholdAvgRatio, fmt.Sprintf("min_threshold.%s.avg_ratio", k), allMetrics[k].minThresholdAvgRatio, fmt.Sprintf("minimum required ratio between average %s metrics", k))
		cmpFlags.Float64Var(&allMetrics[k].minThresholdMaxRatio, fmt.Sprintf("min_threshold.%s.max_ratio", k), allMetrics[k].minThresholdMaxRatio, fmt.Sprintf("minimum required ratio between maximum %s metrics", k))
	}
	cmpFlags.StringSliceVarP(&userMetrics, cfgMetrics, cfgMetricsP, metricNames, "metrics to compare")

	cmpFlags.String(cfgMetricsSourceGitBranch, "", "(optional) git_branch label for the source benchmark instance")
	cmpFlags.String(cfgMetricsTargetGitBranch, "", "(optional) git_branch label for the target benchmark instance")

	// Register all default scenarios and add tests names.
	errors := testCmd.RegisterDefaultScenarios()
	if len(errors) != 0 {
		fmt.Println(errors)
	}
	var tests []string
	for n := range testCmd.Scenarios {
		tests = append(tests, n)
	}
	cmpFlags.StringSliceP(testCmd.CfgTest, testCmd.CfgTestP, tests, "name of e2e test(s) to process")

	// Also take the test parameter flags.
	cmpCmd.Flags().AddFlagSet(testCmd.TestParamsFlags)

	_ = viper.BindPFlags(cmpFlags)
	cmpCmd.Flags().AddFlagSet(cmpFlags)

	parentCmd.AddCommand(cmpCmd)
}

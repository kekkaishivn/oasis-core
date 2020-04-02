package e2e

import (
	"context"
	"fmt"
	flag "github.com/spf13/pflag"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/viper"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/cbor"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/common/sgx"
	"github.com/oasislabs/oasis-core/go/common/sgx/ias"
	cmdCommon "github.com/oasislabs/oasis-core/go/oasis-node/cmd/common"
	cmdNode "github.com/oasislabs/oasis-core/go/oasis-node/cmd/node"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/env"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/log"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis/cli"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario"
	registry "github.com/oasislabs/oasis-core/go/registry/api"
	runtimeClient "github.com/oasislabs/oasis-core/go/runtime/client/api"
	runtimeTransaction "github.com/oasislabs/oasis-core/go/runtime/transaction"
	"github.com/oasislabs/oasis-core/go/storage/database"
)

var (
	// Basic is the basic network + client test case.
	Basic scenario.Scenario = newBasicImpl("basic", "simple-keyvalue-client", nil)
	// BasicEncryption is the basic network + client with encryption test case.
	BasicEncryption scenario.Scenario = newBasicImpl("basic-encryption", "simple-keyvalue-enc-client", nil)

	// DefaultBasicLogWatcherHandlerFactories is a list of default log watcher
	// handler factories for the basic scenario.
	DefaultBasicLogWatcherHandlerFactories = []log.WatcherHandlerFactory{
		oasis.LogAssertNoTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertNoExecutionDiscrepancyDetected(),
		oasis.LogAssertNoMergeDiscrepancyDetected(),
	}
)

func newBasicImpl(name, clientBinary string, clientArgs []string) *basicImpl {
	return &basicImpl{
		name:         name,
		clientBinary: clientBinary,
		clientArgs:   clientArgs,
		logger:       logging.GetLogger("scenario/e2e/" + name),
	}
}

type basicImpl struct {
	net *oasis.Network

	name         string
	clientBinary string
	clientArgs   []string

	logger *logging.Logger
}

func (sc *basicImpl) Clone() scenario.Scenario {
	return &basicImpl{
		name:         sc.name,
		clientBinary: sc.clientBinary,
		clientArgs:   sc.clientArgs,
		logger:       sc.logger,
	}
}

func (sc *basicImpl) Name() string {
	return sc.name
}

func (sc *basicImpl) Parameters() *flag.FlagSet {
	return flag.NewFlagSet(sc.name, flag.ContinueOnError)
}

func (sc *basicImpl) Fixture() (*oasis.NetworkFixture, error) {
	var tee node.TEEHardware
	err := tee.FromString(viper.GetString(cfgTEEHardware))
	if err != nil {
		return nil, err
	}
	var mrSigner *sgx.MrSigner
	if tee == node.TEEHardwareIntelSGX {
		mrSigner = &ias.FortanixTestMrSigner
	}
	keyManagerBinary, err := resolveDefaultKeyManagerBinary()
	if err != nil {
		return nil, err
	}
	runtimeBinary, err := resolveRuntimeBinary("simple-keyvalue")
	if err != nil {
		return nil, err
	}

	return &oasis.NetworkFixture{
		TEE: oasis.TEEFixture{
			Hardware: tee,
			MrSigner: mrSigner,
		},
		Network: oasis.NetworkCfg{
			NodeBinary:                        viper.GetString(cfgNodeBinary),
			RuntimeLoaderBinary:               viper.GetString(cfgRuntimeLoader),
			DefaultLogWatcherHandlerFactories: DefaultBasicLogWatcherHandlerFactories,
		},
		Entities: []oasis.EntityCfg{
			oasis.EntityCfg{IsDebugTestEntity: true},
			oasis.EntityCfg{},
		},
		Runtimes: []oasis.RuntimeFixture{
			// Key manager runtime.
			oasis.RuntimeFixture{
				ID:         keymanagerID,
				Kind:       registry.KindKeyManager,
				Entity:     0,
				Keymanager: -1,
				AdmissionPolicy: registry.RuntimeAdmissionPolicy{
					AnyNode: &registry.AnyNodeRuntimeAdmissionPolicy{},
				},
				Binary: keyManagerBinary,
			},
			// Compute runtime.
			oasis.RuntimeFixture{
				ID:         runtimeID,
				Kind:       registry.KindCompute,
				Entity:     0,
				Keymanager: 0,
				Binary:     runtimeBinary,
				Executor: registry.ExecutorParameters{
					GroupSize:       2,
					GroupBackupSize: 1,
					RoundTimeout:    10 * time.Second,
				},
				Merge: registry.MergeParameters{
					GroupSize:       2,
					GroupBackupSize: 1,
					RoundTimeout:    10 * time.Second,
				},
				TxnScheduler: registry.TxnSchedulerParameters{
					Algorithm:         registry.TxnSchedulerAlgorithmBatching,
					GroupSize:         1,
					MaxBatchSize:      1,
					MaxBatchSizeBytes: 1000,
					BatchFlushTimeout: 1 * time.Second,
				},
				Storage: registry.StorageParameters{
					GroupSize:               2,
					MaxApplyWriteLogEntries: 100_000,
					MaxApplyOps:             2,
					MaxMergeRoots:           8,
					MaxMergeOps:             2,
				},
				AdmissionPolicy: registry.RuntimeAdmissionPolicy{
					AnyNode: &registry.AnyNodeRuntimeAdmissionPolicy{},
				},
			},
		},
		Validators: []oasis.ValidatorFixture{
			oasis.ValidatorFixture{Entity: 1},
			oasis.ValidatorFixture{Entity: 1},
			oasis.ValidatorFixture{Entity: 1},
		},
		Keymanagers: []oasis.KeymanagerFixture{
			oasis.KeymanagerFixture{Runtime: 0, Entity: 1},
		},
		StorageWorkers: []oasis.StorageWorkerFixture{
			oasis.StorageWorkerFixture{Backend: database.BackendNameBadgerDB, Entity: 1},
			oasis.StorageWorkerFixture{Backend: database.BackendNameBadgerDB, Entity: 1},
		},
		ComputeWorkers: []oasis.ComputeWorkerFixture{
			oasis.ComputeWorkerFixture{Entity: 1},
			oasis.ComputeWorkerFixture{Entity: 1},
			oasis.ComputeWorkerFixture{Entity: 1},
		},
		Sentries: []oasis.SentryFixture{},
		Clients: []oasis.ClientFixture{
			oasis.ClientFixture{},
		},
	}, nil
}

func (sc *basicImpl) Init(childEnv *env.Env, net *oasis.Network) error {
	sc.net = net
	return nil
}

func (sc *basicImpl) start(childEnv *env.Env) (<-chan error, *exec.Cmd, error) {
	var err error
	if err = sc.net.Start(); err != nil {
		return nil, nil, err
	}

	cmd, err := startClient(childEnv, sc.net, resolveClientBinary(sc.clientBinary), sc.clientArgs)
	if err != nil {
		return nil, nil, err
	}

	clientErrCh := make(chan error)
	go func() {
		clientErrCh <- cmd.Wait()
	}()
	return clientErrCh, cmd, nil
}

func (sc *basicImpl) cleanTendermintStorage(childEnv *env.Env) error {
	doClean := func(dataDir string, cleanArgs []string) error {
		args := append([]string{
			"unsafe-reset",
			"--" + cmdCommon.CfgDataDir, dataDir,
		}, cleanArgs...)

		return cli.RunSubCommand(childEnv, logger, "unsafe-reset", sc.net.Config().NodeBinary, args)
	}

	for _, val := range sc.net.Validators() {
		if err := doClean(val.DataDir(), nil); err != nil {
			return err
		}
	}
	for _, cw := range sc.net.ComputeWorkers() {
		if err := doClean(cw.DataDir(), nil); err != nil {
			return err
		}
	}
	for _, cl := range sc.net.Clients() {
		if err := doClean(cl.DataDir(), nil); err != nil {
			return err
		}
	}
	for _, bz := range sc.net.Byzantine() {
		if err := doClean(bz.DataDir(), nil); err != nil {
			return err
		}
	}
	for _, se := range sc.net.Sentries() {
		if err := doClean(se.DataDir(), nil); err != nil {
			return err
		}
	}
	for _, sw := range sc.net.StorageWorkers() {
		if err := doClean(sw.DataDir(), []string{"--" + cmdNode.CfgPreserveMKVSDatabase}); err != nil {
			return err
		}
	}
	for _, kw := range sc.net.Keymanagers() {
		if err := doClean(kw.DataDir(), []string{"--" + cmdNode.CfgPreserveLocalStorage}); err != nil {
			return err
		}
	}

	return nil
}

func (sc *basicImpl) dumpRestoreNetwork(childEnv *env.Env, fixture *oasis.NetworkFixture) error {
	// Dump-restore network.
	sc.logger.Info("dumping network state",
		"child", childEnv,
	)

	dumpPath := filepath.Join(childEnv.Dir(), "genesis_dump.json")
	args := []string{
		"genesis", "dump",
		"--height", "0",
		"--genesis.file", dumpPath,
		"--address", "unix:" + sc.net.Validators()[0].SocketPath(),
	}

	if err := cli.RunSubCommand(childEnv, logger, "genesis-dump", sc.net.Config().NodeBinary, args); err != nil {
		return fmt.Errorf("scenario/e2e/dump_restore: failed to dump state: %w", err)
	}

	// Stop the network.
	logger.Info("stopping the network")
	sc.net.Stop()

	if len(sc.net.StorageWorkers()) > 0 {
		// Dump storage.
		args = []string{
			"debug", "storage", "export",
			"--genesis.file", dumpPath,
			"--datadir", sc.net.StorageWorkers()[0].DataDir(),
			"--storage.export.dir", filepath.Join(childEnv.Dir(), "storage_dumps"),
			"--debug.dont_blame_oasis",
			"--debug.allow_test_keys",
		}
		if err := cli.RunSubCommand(childEnv, logger, "storage-dump", sc.net.Config().NodeBinary, args); err != nil {
			return fmt.Errorf("scenario/e2e/dump_restore: failed to dump storage: %w", err)
		}
	}

	// Reset all the state back to the vanilla state.
	if err := sc.cleanTendermintStorage(childEnv); err != nil {
		return fmt.Errorf("scenario/e2e/dump_restore: failed to clean tendemint storage: %w", err)
	}

	// Start the network and the client again.
	logger.Info("starting the network again")

	fixture.Network.GenesisFile = dumpPath
	// Make sure to not overwrite entities.
	for i, entity := range fixture.Entities {
		if !entity.IsDebugTestEntity {
			fixture.Entities[i].Restore = true
		}
	}

	var err error
	if sc.net, err = fixture.Create(childEnv); err != nil {
		return err
	}

	return nil
}

func (sc *basicImpl) finishWithoutChild() error {
	var err error
	select {
	case err = <-sc.net.Errors():
		return err
	default:
		return sc.net.CheckLogWatchers()
	}
}

func (sc *basicImpl) wait(childEnv *env.Env, cmd *exec.Cmd, clientErrCh <-chan error) error {
	var err error
	select {
	case err = <-sc.net.Errors():
		_ = cmd.Process.Kill()
	case err = <-clientErrCh:
	}
	if err != nil {
		return err
	}

	if err = sc.net.CheckLogWatchers(); err != nil {
		return err
	}

	return nil
}

func (sc *basicImpl) Run(childEnv *env.Env) error {
	clientErrCh, cmd, err := sc.start(childEnv)
	if err != nil {
		return err
	}

	return sc.wait(childEnv, cmd, clientErrCh)
}

func (sc *basicImpl) submitRuntimeTx(ctx context.Context, id common.Namespace, key, value string) error {
	c := sc.net.ClientController().RuntimeClient

	// Submit a transaction and check the result.
	var rsp runtimeTransaction.TxnOutput
	rawRsp, err := c.SubmitTx(ctx, &runtimeClient.SubmitTxRequest{
		RuntimeID: id,
		Data: cbor.Marshal(&runtimeTransaction.TxnCall{
			Method: "insert",
			Args: struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}{
				Key:   key,
				Value: value,
			},
		}),
	})
	if err != nil {
		return fmt.Errorf("failed to submit runtime tx: %w", err)
	}
	if err = cbor.Unmarshal(rawRsp, &rsp); err != nil {
		return fmt.Errorf("malformed tx output from runtime: %w", err)
	}
	if rsp.Error != nil {
		return fmt.Errorf("runtime tx failed: %s", *rsp.Error)
	}
	return nil
}

func (sc *basicImpl) waitNodesSynced() error {
	ctx := context.Background()

	checkSynced := func(n *oasis.Node) error {
		c, err := oasis.NewController(n.SocketPath())
		if err != nil {
			return fmt.Errorf("failed to create node controller: %w", err)
		}
		defer c.Close()

		if err = c.WaitSync(ctx); err != nil {
			return fmt.Errorf("failed to wait for node to sync: %w", err)
		}
		return nil
	}

	sc.logger.Info("waiting for all nodes to be synced")

	for _, n := range sc.net.Validators() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.net.StorageWorkers() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.net.ComputeWorkers() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}
	for _, n := range sc.net.Clients() {
		if err := checkSynced(&n.Node); err != nil {
			return err
		}
	}

	sc.logger.Info("nodes synced")
	return nil
}

func (sc *basicImpl) initialEpochTransitions() error {
	ctx := context.Background()

	if len(sc.net.Keymanagers()) > 0 {
		// First wait for validator and key manager nodes to register. Then perform an epoch
		// transition which will cause the compute and storage nodes to register.
		numNodes := len(sc.net.Validators()) + len(sc.net.Keymanagers())
		sc.logger.Info("waiting for (some) nodes to register",
			"num_nodes", numNodes,
		)

		if err := sc.net.Controller().WaitNodesRegistered(ctx, numNodes); err != nil {
			return fmt.Errorf("failed to wait for nodes: %w", err)
		}

		sc.logger.Info("triggering epoch transition")
		if err := sc.net.Controller().SetEpoch(ctx, 1); err != nil {
			return fmt.Errorf("failed to set epoch: %w", err)
		}
		sc.logger.Info("epoch transition done")
	}

	// Wait for all nodes to register.
	sc.logger.Info("waiting for (all) nodes to register",
		"num_nodes", sc.net.NumRegisterNodes(),
	)

	if err := sc.net.Controller().WaitNodesRegistered(ctx, sc.net.NumRegisterNodes()); err != nil {
		return fmt.Errorf("failed to wait for nodes: %w", err)
	}

	// Then perform another epoch transition to elect the committees.
	sc.logger.Info("triggering epoch transition")
	if err := sc.net.Controller().SetEpoch(ctx, 2); err != nil {
		return fmt.Errorf("failed to set epoch: %w", err)
	}
	sc.logger.Info("epoch transition done")
	return nil
}

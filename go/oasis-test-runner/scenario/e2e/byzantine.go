package e2e

import (
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/env"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/log"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/oasis"
	"github.com/oasislabs/oasis-core/go/oasis-test-runner/scenario"
)

// TODO: Consider referencing script names directly from the Byzantine node.

var (
	// Permutations generated in the epoch 2 election are
	// executor:              3 (w), 0 (w), 2 (b), 1 (i)
	// transaction scheduler: 0 (w), 3 (i), 1 (i), 2 (i)
	// merge:                 1 (w), 2 (w), 0 (b), 3 (i)
	// w = worker; b = backup; i = invalid
	// For executor scripts, it suffices to be index 3.
	// For merge scripts, it suffices to be index 1.
	// No index is transaction scheduler only.
	// Indices are by order of node ID.

	// ByzantineExecutorHonest is the byzantine executor honest scenario.
	ByzantineExecutorHonest scenario.Scenario = newByzantineImpl("executor-honest", nil, oasis.ByzantineSlot3IdentitySeed)
	// ByzantineExecutorWrong is the byzantine executor wrong scenario.
	ByzantineExecutorWrong scenario.Scenario = newByzantineImpl("executor-wrong", []log.WatcherHandlerFactory{
		oasis.LogAssertNoTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertExecutionDiscrepancyDetected(),
		oasis.LogAssertNoMergeDiscrepancyDetected(),
	}, oasis.ByzantineSlot3IdentitySeed)
	// ByzantineExecutorStraggler is the byzantine executor straggler scenario.
	ByzantineExecutorStraggler scenario.Scenario = newByzantineImpl("executor-straggler", []log.WatcherHandlerFactory{
		oasis.LogAssertTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertExecutionDiscrepancyDetected(),
		oasis.LogAssertNoMergeDiscrepancyDetected(),
	}, oasis.ByzantineSlot3IdentitySeed)

	// ByzantineMergeHonest is the byzantine merge honest scenario.
	ByzantineMergeHonest scenario.Scenario = newByzantineImpl("merge-honest", nil, oasis.ByzantineSlot1IdentitySeed)
	// ByzantineMergeWrong is the byzantine merge wrong scenario.
	ByzantineMergeWrong scenario.Scenario = newByzantineImpl("merge-wrong", []log.WatcherHandlerFactory{
		oasis.LogAssertNoTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertNoExecutionDiscrepancyDetected(),
		oasis.LogAssertMergeDiscrepancyDetected(),
	}, oasis.ByzantineSlot1IdentitySeed)
	// ByzantineMergeStraggler is the byzantine merge straggler scenario.
	ByzantineMergeStraggler scenario.Scenario = newByzantineImpl("merge-straggler", []log.WatcherHandlerFactory{
		oasis.LogAssertTimeouts(),
		oasis.LogAssertNoRoundFailures(),
		oasis.LogAssertNoExecutionDiscrepancyDetected(),
		oasis.LogAssertMergeDiscrepancyDetected(),
	}, oasis.ByzantineSlot1IdentitySeed)
)

type byzantineImpl struct {
	basicImpl

	script                     string
	identitySeed               string
	logWatcherHandlerFactories []log.WatcherHandlerFactory
}

func newByzantineImpl(script string, logWatcherHandlerFactories []log.WatcherHandlerFactory, identitySeed string) scenario.Scenario {
	return &byzantineImpl{
		basicImpl: *newBasicImpl(
			"byzantine/"+script,
			"simple-keyvalue-ops-client",
			[]string{"set", "hello_key", "hello_value"},
		),
		script:                     script,
		identitySeed:               identitySeed,
		logWatcherHandlerFactories: logWatcherHandlerFactories,
	}
}

func (sc *byzantineImpl) Clone() scenario.Scenario {
	return &byzantineImpl{
		basicImpl:                  *sc.basicImpl.Clone().(*basicImpl),
		script:                     sc.script,
		identitySeed:               sc.identitySeed,
		logWatcherHandlerFactories: sc.logWatcherHandlerFactories,
	}
}

func (sc *byzantineImpl) Fixture() (*oasis.NetworkFixture, error) {
	f, err := sc.basicImpl.Fixture()
	if err != nil {
		return nil, err
	}

	// The byzantine node requires deterministic identities.
	f.Network.DeterministicIdentities = true
	// The byzantine scenario requires mock epochtime as the byzantine node
	// doesn't know how to handle epochs in which it is not scheduled.
	f.Network.EpochtimeMock = true
	// Change the default network log watcher handler factories if configured.
	if sc.logWatcherHandlerFactories != nil {
		f.Network.DefaultLogWatcherHandlerFactories = sc.logWatcherHandlerFactories
	}
	// Provision a Byzantine node.
	f.ByzantineNodes = []oasis.ByzantineFixture{
		oasis.ByzantineFixture{
			Script:          sc.script,
			IdentitySeed:    sc.identitySeed,
			Entity:          1,
			ActivationEpoch: 1,
		},
	}
	return f, nil
}

func (sc *byzantineImpl) Run(childEnv *env.Env) error {
	clientErrCh, cmd, err := sc.basicImpl.start(childEnv)
	if err != nil {
		return err
	}

	if err = sc.initialEpochTransitions(); err != nil {
		return err
	}

	return sc.wait(childEnv, cmd, clientErrCh)
}

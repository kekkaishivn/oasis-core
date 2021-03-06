package scheduler

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/tendermint/tendermint/abci/types"

	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/node"
	consensus "github.com/oasislabs/oasis-core/go/consensus/api"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/abci"
	registryState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/registry/state"
	schedulerState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/scheduler/state"
	genesis "github.com/oasislabs/oasis-core/go/genesis/api"
	scheduler "github.com/oasislabs/oasis-core/go/scheduler/api"
)

func (app *schedulerApplication) InitChain(ctx *abci.Context, req types.RequestInitChain, doc *genesis.Document) error {
	baseEpoch, err := app.state.GetBaseEpoch()
	if err != nil {
		return fmt.Errorf("tendermint/scheduler: couldn't get base epoch: %w", err)
	}
	app.baseEpoch = baseEpoch

	state := schedulerState.NewMutableState(ctx.State())
	state.SetConsensusParameters(&doc.Scheduler.Parameters)

	if doc.Scheduler.Parameters.DebugStaticValidators {
		ctx.Logger().Warn("static validators are configured")

		var staticValidators []signature.PublicKey
		for _, v := range req.Validators {
			tmPk := v.GetPubKey()

			if t := tmPk.GetType(); t != types.PubKeyEd25519 {
				ctx.Logger().Error("invalid static validator public key type",
					"public_key", hex.EncodeToString(tmPk.GetData()),
					"type", t,
				)
				return fmt.Errorf("scheduler: invalid static validator public key type: '%v'", t)
			}

			var id signature.PublicKey
			if err = id.UnmarshalBinary(tmPk.GetData()); err != nil {
				ctx.Logger().Error("invalid static validator public key",
					"err", err,
					"public_key", hex.EncodeToString(tmPk.GetData()),
				)
				return fmt.Errorf("scheduler: invalid static validator public key: %w", err)
			}

			staticValidators = append(staticValidators, id)
		}

		// Add the current validator set to ABCI, so that we can query it later.
		state.PutCurrentValidators(staticValidators)

		return nil
	}

	if doc.Scheduler.Parameters.MinValidators <= 0 {
		return fmt.Errorf("tendermint/scheduler: minimum number of validators not configured")
	}
	if doc.Scheduler.Parameters.MaxValidators <= 0 {
		return fmt.Errorf("tendermint/scheduler: maximum number of validators not configured")
	}
	if doc.Scheduler.Parameters.MaxValidatorsPerEntity <= 0 {
		return fmt.Errorf("tendermint/scheduler: maximum number of validators per entity not configured")
	}
	if doc.Scheduler.Parameters.MaxValidatorsPerEntity > 1 {
		// This should only ever be true for test deployments.
		ctx.Logger().Warn("maximum number of validators is non-standard, fairness not guaranteed",
			"max_valiators_per_entity", doc.Scheduler.Parameters.MaxValidatorsPerEntity,
		)
	}

	regState := registryState.NewMutableState(ctx.State())
	nodes, err := regState.Nodes()
	if err != nil {
		return fmt.Errorf("tendermint/scheduler: couldn't get nodes: %w", err)
	}

	registeredValidators := make(map[signature.PublicKey]*node.Node)
	for _, v := range nodes {
		if v.HasRoles(node.RoleValidator) {
			registeredValidators[v.Consensus.ID] = v
		}
	}

	// Assemble the list of the tendermint genesis validators, and do some
	// sanity checking.
	var currentValidators []signature.PublicKey
	for _, v := range req.Validators {
		tmPk := v.GetPubKey()

		if t := tmPk.GetType(); t != types.PubKeyEd25519 {
			ctx.Logger().Error("invalid genesis validator public key type",
				"public_key", hex.EncodeToString(tmPk.GetData()),
				"type", t,
			)
			return fmt.Errorf("scheduler: invalid genesis validator public key type: '%v'", t)
		}

		var id signature.PublicKey
		if err = id.UnmarshalBinary(tmPk.GetData()); err != nil {
			ctx.Logger().Error("invalid genesis validator public key",
				"err", err,
				"public_key", hex.EncodeToString(tmPk.GetData()),
			)
			return fmt.Errorf("scheduler: invalid genesis validator public key: %w", err)
		}

		if power := v.GetPower(); power != consensus.VotingPower {
			ctx.Logger().Error("invalid voting power",
				"id", id,
				"power", power,
			)
			return fmt.Errorf("scheduler: invalid genesis validator voting power: %v", power)
		}

		n := registeredValidators[id]
		if n == nil {
			ctx.Logger().Error("genesis validator not in registry",
				"id", id,
			)
			return fmt.Errorf("scheduler: genesis validator not in registry")
		}
		ctx.Logger().Debug("adding validator to current validator set",
			"id", id,
		)
		currentValidators = append(currentValidators, n.Consensus.ID)
	}

	// TODO/security: Enforce genesis validator staking.

	// Add the current validator set to ABCI, so that we can alter it later.
	//
	// Sort of stupid it needs to be done this way, but tendermint doesn't
	// appear to pass ABCI the validator set anywhere other than InitChain.
	state.PutCurrentValidators(currentValidators)

	return nil
}

func (sq *schedulerQuerier) Genesis(ctx context.Context) (*scheduler.Genesis, error) {
	params, err := sq.state.ConsensusParameters()
	if err != nil {
		return nil, err
	}

	genesis := &scheduler.Genesis{
		Parameters: *params,
	}
	return genesis, nil
}

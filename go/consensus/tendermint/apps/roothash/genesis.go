package roothash

import (
	"context"
	"fmt"

	"github.com/tendermint/tendermint/abci/types"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	abciAPI "github.com/oasislabs/oasis-core/go/consensus/tendermint/api"
	registryState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/registry/state"
	roothashState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/roothash/state"
	genesisAPI "github.com/oasislabs/oasis-core/go/genesis/api"
	"github.com/oasislabs/oasis-core/go/registry/api"
	roothashAPI "github.com/oasislabs/oasis-core/go/roothash/api"
	storageAPI "github.com/oasislabs/oasis-core/go/storage/api"
)

func (app *rootHashApplication) InitChain(ctx *abciAPI.Context, request types.RequestInitChain, doc *genesisAPI.Document) error {
	st := doc.RootHash

	state := roothashState.NewMutableState(ctx.State())
	if err := state.SetConsensusParameters(ctx, &st.Parameters); err != nil {
		return fmt.Errorf("failed to set consensus parameters: %w", err)
	}

	// The per-runtime roothash state is done primarily via DeliverTx, but
	// also needs to be done here since the genesis state can have runtime
	// registrations.
	//
	// Note: This could use the genesis state, but the registry has already
	// carved out it's entries by this point.

	regState := registryState.NewMutableState(ctx.State())
	runtimes, _ := regState.Runtimes(ctx)
	for _, v := range runtimes {
		ctx.Logger().Info("InitChain: allocating per-runtime state",
			"runtime", v.ID,
		)
		if err := app.onNewRuntime(ctx, v, &st); err != nil {
			return fmt.Errorf("failed to initialize runtime %s state: %w", v.ID, err)
		}
	}

	return nil
}

func (rq *rootHashQuerier) Genesis(ctx context.Context) (*roothashAPI.Genesis, error) {
	runtimes, err := rq.state.Runtimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch runtimes: %w", err)
	}

	// Get per-runtime states.
	rtStates := make(map[common.Namespace]*api.RuntimeGenesis)
	for _, rt := range runtimes {
		rtState := api.RuntimeGenesis{
			StateRoot: rt.CurrentBlock.Header.StateRoot,
			// State is always empty in Genesis regardless of StateRoot.
			State:           storageAPI.WriteLog{},
			StorageReceipts: []signature.Signature{},
			Round:           rt.CurrentBlock.Header.Round,
		}

		rtStates[rt.Runtime.ID] = &rtState
	}

	params, err := rq.state.ConsensusParameters(ctx)
	if err != nil {
		return nil, err
	}

	genesis := &roothashAPI.Genesis{
		Parameters:    *params,
		RuntimeStates: rtStates,
	}
	return genesis, nil
}

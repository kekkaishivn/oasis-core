package registry

import (
	"context"
	"fmt"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/entity"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/consensus/tendermint/abci"
	registryState "github.com/oasislabs/oasis-core/go/consensus/tendermint/apps/registry/state"
	registry "github.com/oasislabs/oasis-core/go/registry/api"
)

// Query is the registry query interface.
type Query interface {
	Entity(context.Context, signature.PublicKey) (*entity.Entity, error)
	Entities(context.Context) ([]*entity.Entity, error)
	Node(context.Context, signature.PublicKey) (*node.Node, error)
	NodeStatus(context.Context, signature.PublicKey) (*registry.NodeStatus, error)
	Nodes(context.Context) ([]*node.Node, error)
	Runtime(context.Context, common.Namespace) (*registry.Runtime, error)
	Runtimes(context.Context) ([]*registry.Runtime, error)
	Genesis(context.Context) (*registry.Genesis, error)
}

// QueryFactory is the registry query factory.
type QueryFactory struct {
	app *registryApplication
}

// ImmutableStateAt creates a registry ImmutableState suitable for use
// from external query instances.
func ImmutableStateAt(ctx context.Context, appState abci.ApplicationState, height int64) (*registryState.ImmutableState, error) {
	var (
		state *registryState.ImmutableState
		err   error
	)
	abciCtx := abci.FromCtx(ctx)

	// If this request was made from InitChain, no blocks and states have been
	// submitted yet, so we use the existing state instead.
	if abciCtx != nil && abciCtx.IsInitChain() {
		state = registryState.NewMutableState(abciCtx.State()).ImmutableState
	} else {
		state, err = registryState.NewImmutableState(appState, height)
		if err != nil {
			return nil, err
		}
	}

	// If this request was made from an ABCI app, make sure to use the associated
	// context for querying state instead of the default one.
	if abciCtx != nil && height == abciCtx.BlockHeight()+1 {
		state.Snapshot = abciCtx.State().ImmutableTree
	}

	return state, nil
}

// QueryAt returns the registry query interface for a specific height.
func (sf *QueryFactory) QueryAt(ctx context.Context, height int64) (Query, error) {
	state, err := ImmutableStateAt(ctx, sf.app.state, height)
	if err != nil {
		return nil, err
	}

	return &registryQuerier{sf.app, state, height}, nil
}

type registryQuerier struct {
	app    *registryApplication
	state  *registryState.ImmutableState
	height int64
}

func (rq *registryQuerier) Entity(ctx context.Context, id signature.PublicKey) (*entity.Entity, error) {
	return rq.state.Entity(id)
}

func (rq *registryQuerier) Entities(ctx context.Context) ([]*entity.Entity, error) {
	return rq.state.Entities()
}

func (rq *registryQuerier) Node(ctx context.Context, id signature.PublicKey) (*node.Node, error) {
	epoch, err := rq.app.state.GetEpoch(ctx, rq.height)
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch: %w", err)
	}

	node, err := rq.state.Node(id)
	if err != nil {
		return nil, err
	}

	// Do not return expired nodes.
	if node.IsExpired(uint64(epoch)) {
		return nil, registry.ErrNoSuchNode
	}
	return node, nil
}

func (rq *registryQuerier) NodeStatus(ctx context.Context, id signature.PublicKey) (*registry.NodeStatus, error) {
	return rq.state.NodeStatus(id)
}

func (rq *registryQuerier) Nodes(ctx context.Context) ([]*node.Node, error) {
	epoch, err := rq.app.state.GetEpoch(ctx, rq.height)
	if err != nil {
		return nil, fmt.Errorf("failed to get epoch: %w", err)
	}

	nodes, err := rq.state.Nodes()
	if err != nil {
		return nil, err
	}

	// Filter out expired nodes.
	var filteredNodes []*node.Node
	for _, n := range nodes {
		if n.IsExpired(uint64(epoch)) {
			continue
		}
		filteredNodes = append(filteredNodes, n)
	}
	return filteredNodes, nil
}

func (rq *registryQuerier) Runtime(ctx context.Context, id common.Namespace) (*registry.Runtime, error) {
	return rq.state.Runtime(id)
}

func (rq *registryQuerier) Runtimes(ctx context.Context) ([]*registry.Runtime, error) {
	return rq.state.Runtimes()
}

func (app *registryApplication) QueryFactory() interface{} {
	return &QueryFactory{app}
}

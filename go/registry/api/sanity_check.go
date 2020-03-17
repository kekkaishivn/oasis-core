package api

import (
	"fmt"
	"time"

	"github.com/oasislabs/oasis-core/go/common"
	"github.com/oasislabs/oasis-core/go/common/crypto/hash"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/entity"
	"github.com/oasislabs/oasis-core/go/common/logging"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/common/quantity"
	epochtime "github.com/oasislabs/oasis-core/go/epochtime/api"
	"github.com/oasislabs/oasis-core/go/oasis-node/cmd/common/flags"
	staking "github.com/oasislabs/oasis-core/go/staking/api"
)

// SanityCheck does basic sanity checking on the genesis state.
func (g *Genesis) SanityCheck(
	baseEpoch epochtime.EpochTime,
	stakeLedger map[signature.PublicKey]*staking.Account,
	stakeThresholds map[staking.ThresholdKind]quantity.Quantity,
) error {
	logger := logging.GetLogger("genesis/sanity-check")

	if !flags.DebugDontBlameOasis() {
		if g.Parameters.DebugAllowUnroutableAddresses || g.Parameters.DebugBypassStake || g.Parameters.DebugAllowEntitySignedNodeRegistration {
			return fmt.Errorf("registry: sanity check failed: one or more unsafe debug flags set")
		}
		if g.Parameters.MaxNodeExpiration == 0 {
			return fmt.Errorf("registry: sanity check failed: maximum node expiration not specified")
		}
	}

	// Check entities.
	seenEntities, err := SanityCheckEntities(logger, g.Entities)
	if err != nil {
		return err
	}

	// Check runtimes.
	runtimesLookup, err := SanityCheckRuntimes(logger, &g.Parameters, g.Runtimes, g.SuspendedRuntimes, true)
	if err != nil {
		return err
	}

	// Check nodes.
	nodeLookup, err := SanityCheckNodes(logger, &g.Parameters, g.Nodes, seenEntities, runtimesLookup, true, baseEpoch)
	if err != nil {
		return err
	}

	if !g.Parameters.DebugBypassStake {
		entities := []*entity.Entity{}
		for _, ent := range seenEntities {
			entities = append(entities, ent)
		}
		runtimes, err := runtimesLookup.AllRuntimes()
		if err != nil {
			return fmt.Errorf("registry: sanity check failed: could not obtain all runtimes from runtimesLookup: %w", err)
		}
		nodes, err := nodeLookup.Nodes()
		if err != nil {
			return fmt.Errorf("registry: sanity check failed: could not obtain node list from nodeLookup: %w", err)
		}
		// Check stake.
		return SanityCheckStake(entities, stakeLedger, nodes, runtimes, stakeThresholds)
	}

	return nil
}

// SanityCheckEntities examines the entities table.
// Returns lookup of entity ID to the entity record for use in other checks.
func SanityCheckEntities(logger *logging.Logger, entities []*entity.SignedEntity) (map[signature.PublicKey]*entity.Entity, error) {
	seenEntities := make(map[signature.PublicKey]*entity.Entity)
	for _, signedEnt := range entities {
		entity, err := VerifyRegisterEntityArgs(logger, signedEnt, true)
		if err != nil {
			return nil, fmt.Errorf("entity sanity check failed: %w", err)
		}
		seenEntities[entity.ID] = entity
	}

	return seenEntities, nil
}

// SanityCheckRuntimes examines the runtimes table.
func SanityCheckRuntimes(logger *logging.Logger,
	params *ConsensusParameters,
	runtimes []*SignedRuntime,
	suspendedRuntimes []*SignedRuntime,
	isGenesis bool,
) (RuntimeLookup, error) {
	// First go through all runtimes and perform general sanity checks.
	seenRuntimes := []*Runtime{}
	for _, signedRt := range runtimes {
		rt, err := VerifyRegisterRuntimeArgs(params, logger, signedRt, isGenesis)
		if err != nil {
			return nil, fmt.Errorf("runtime sanity check failed: %w", err)
		}
		seenRuntimes = append(seenRuntimes, rt)
	}

	seenSuspendedRuntimes := []*Runtime{}
	for _, signedRt := range suspendedRuntimes {
		rt, err := VerifyRegisterRuntimeArgs(params, logger, signedRt, isGenesis)
		if err != nil {
			return nil, fmt.Errorf("runtime sanity check failed: %w", err)
		}
		seenSuspendedRuntimes = append(seenSuspendedRuntimes, rt)
	}

	// Then build a runtime lookup table and re-check compute runtimes as those need to reference
	// correct key manager runtimes when a key manager is configured.
	lookup, err := newSanityCheckRuntimeLookup(seenRuntimes, seenSuspendedRuntimes)
	if err != nil {
		return nil, fmt.Errorf("runtime sanity check failed: %w", err)
	}
	for _, runtimes := range [][]*Runtime{seenRuntimes, seenSuspendedRuntimes} {
		for _, rt := range runtimes {
			if rt.Kind != KindCompute {
				continue
			}
			if err := VerifyRegisterComputeRuntimeArgs(logger, rt, lookup); err != nil {
				return nil, fmt.Errorf("compute runtime sanity check failed: %w", err)
			}
		}
	}
	return lookup, nil
}

// SanityCheckNodes examines the nodes table.
// Pass lookups of entities and runtimes from SanityCheckEntities
// and SanityCheckRuntimes for cross referencing purposes.
func SanityCheckNodes(
	logger *logging.Logger,
	params *ConsensusParameters,
	nodes []*node.MultiSignedNode,
	seenEntities map[signature.PublicKey]*entity.Entity,
	runtimesLookup RuntimeLookup,
	isGenesis bool,
	epoch epochtime.EpochTime,
) (NodeLookup, error) { // nolint: gocyclo

	nodeLookup := &sanityCheckNodeLookup{
		nodes:           make(map[signature.PublicKey]*node.Node),
		nodesCertHashes: make(map[hash.Hash]*node.Node),
	}

	for _, signedNode := range nodes {

		// Open the node to get the referenced entity.
		var n node.Node
		if err := signedNode.Open(RegisterGenesisNodeSignatureContext, &n); err != nil {
			return nil, fmt.Errorf("registry: sanity check failed: unable to open signed node")
		}
		if !n.ID.IsValid() {
			return nil, fmt.Errorf("registry: node sanity check failed: ID %s is invalid", n.ID.String())
		}
		entity, ok := seenEntities[n.EntityID]
		if !ok {
			return nil, fmt.Errorf("registry: node sanity check failed node: %s references a missing entity", n.ID.String())
		}

		node, _, err := VerifyRegisterNodeArgs(params,
			logger,
			signedNode,
			entity,
			time.Now(),
			isGenesis,
			epoch,
			runtimesLookup,
			nodeLookup,
		)
		if err != nil {
			return nil, fmt.Errorf("registry: node sanity check failed: ID: %s, error: %w", n.ID.String(), err)
		}

		// Add validated node to nodeLookup.
		nodeLookup.nodes[node.Consensus.ID] = node
		nodeLookup.nodes[node.P2P.ID] = node
		nodeLookup.nodesList = append(nodeLookup.nodesList, node)

		var h = hash.Hash{}
		h.FromBytes(node.Committee.Certificate)
		nodeLookup.nodesCertHashes[h] = node
	}

	return nodeLookup, nil
}

// SanityCheckStake ensures entities' stake accumulator claims are consistent
// with general state and entities have enough stake for themselves and all
// their registered nodes and runtimes.
func SanityCheckStake(
	entities []*entity.Entity,
	accounts map[signature.PublicKey]*staking.Account,
	nodes []*node.Node,
	runtimes []*Runtime,
	stakeThresholds map[staking.ThresholdKind]quantity.Quantity,
) error {
	// Entities' escrow accounts for checking claims and stake.
	generatedEscrows := make(map[signature.PublicKey]*staking.EscrowAccount)

	// Generate escrow account for all entities.
	for _, entity := range entities {
		acct, ok := accounts[entity.ID]
		if !ok {
			return fmt.Errorf("registry: sanity check failed: no account associated with entity with ID: %v", entity.ID)
		}

		// Generate an escrow account with the same active balance and shares number.
		escrow := &staking.EscrowAccount{
			Active: staking.SharePool{
				Balance:     acct.Escrow.Active.Balance,
				TotalShares: acct.Escrow.Active.TotalShares,
			},
		}

		// Add entity stake claim.
		escrow.StakeAccumulator.AddClaimUnchecked(StakeClaimRegisterEntity, []staking.ThresholdKind{staking.KindEntity})

		generatedEscrows[entity.ID] = escrow
	}

	for _, node := range nodes {
		// Add node stake claims.
		generatedEscrows[node.EntityID].StakeAccumulator.AddClaimUnchecked(StakeClaimForNode(node.ID), StakeThresholdsForNode(node))
	}
	for _, rt := range runtimes {
		// Add runtime stake claims.
		generatedEscrows[rt.EntityID].StakeAccumulator.AddClaimUnchecked(StakeClaimForRuntime(rt.ID), StakeThresholdsForRuntime(rt))
	}

	// Compare entities' generated escrow accounts with actual ones.
	for _, entity := range entities {
		generatedEscrow := generatedEscrows[entity.ID]
		actualEscrow := accounts[entity.ID].Escrow

		// Compare expected accumulator state with the actual one.
		expectedClaims := generatedEscrows[entity.ID].StakeAccumulator.Claims
		actualClaims := actualEscrow.StakeAccumulator.Claims
		if len(expectedClaims) != len(actualClaims) {
			return fmt.Errorf("incorrect number of stake claims for account %s (expected: %d got: %d)",
				entity.ID,
				len(expectedClaims),
				len(actualClaims),
			)
		}
		for claim, expectedThresholds := range expectedClaims {
			thresholds, ok := actualClaims[claim]
			if !ok {
				return fmt.Errorf("missing claim %s for account %s", claim, entity.ID)
			}
			if len(thresholds) != len(expectedThresholds) {
				return fmt.Errorf("incorrect number of thresholds for claim %s for account %s (expected: %d got: %d)",
					claim,
					entity.ID,
					len(expectedThresholds),
					len(thresholds),
				)
			}
			for i, expectedThreshold := range expectedThresholds {
				threshold := thresholds[i]
				if threshold != expectedThreshold {
					return fmt.Errorf("incorrect threshold in position %d for claim %s for account %s (expected: %s got: %s)",
						i,
						claim,
						entity.ID,
						expectedThreshold,
						threshold,
					)
				}
			}
		}

		// Check if entity has enough stake for all stake claims.
		if err := generatedEscrow.CheckStakeClaims(stakeThresholds); err != nil {
			var expected = "unknown"
			expectedQty, err := generatedEscrow.StakeAccumulator.TotalClaims(stakeThresholds, nil)
			if err == nil {
				expected = expectedQty.String()
			}
			return fmt.Errorf("insufficient stake for account %s (expected: %s got: %s)", entity.ID, expected, generatedEscrow.Active.Balance)
		}
	}

	return nil
}

// Runtimes lookup used in sanity checks.
type sanityCheckRuntimeLookup struct {
	runtimes          map[common.Namespace]*Runtime
	suspendedRuntimes map[common.Namespace]*Runtime
	allRuntimes       []*Runtime
}

func newSanityCheckRuntimeLookup(runtimes []*Runtime, suspendedRuntimes []*Runtime) (RuntimeLookup, error) {
	rtsMap := make(map[common.Namespace]*Runtime)
	sRtsMap := make(map[common.Namespace]*Runtime)
	allRts := []*Runtime{}
	for _, rt := range runtimes {
		if rtsMap[rt.ID] != nil {
			return nil, fmt.Errorf("duplicate runtime: %s", rt.ID)
		}
		rtsMap[rt.ID] = rt
		allRts = append(allRts, rt)
	}
	for _, srt := range suspendedRuntimes {
		if rtsMap[srt.ID] != nil || sRtsMap[srt.ID] != nil {
			return nil, fmt.Errorf("duplicate (suspended) runtime: %s", srt.ID)
		}
		sRtsMap[srt.ID] = srt
		allRts = append(allRts, srt)
	}
	return &sanityCheckRuntimeLookup{
		runtimes:          rtsMap,
		suspendedRuntimes: sRtsMap,
		allRuntimes:       allRts,
	}, nil
}

func (r *sanityCheckRuntimeLookup) Runtime(id common.Namespace) (*Runtime, error) {
	rt, ok := r.runtimes[id]
	if !ok {
		return nil, fmt.Errorf("runtime not found")
	}
	return rt, nil
}

func (r *sanityCheckRuntimeLookup) SuspendedRuntime(id common.Namespace) (*Runtime, error) {
	srt, ok := r.suspendedRuntimes[id]
	if !ok {
		return nil, ErrNoSuchRuntime
	}
	return srt, nil
}

func (r *sanityCheckRuntimeLookup) AnyRuntime(id common.Namespace) (*Runtime, error) {
	rt, ok := r.runtimes[id]
	if !ok {
		srt, ok := r.suspendedRuntimes[id]
		if !ok {
			return nil, ErrNoSuchRuntime
		}
		return srt, nil
	}
	return rt, nil
}

func (r *sanityCheckRuntimeLookup) AllRuntimes() ([]*Runtime, error) {
	return r.allRuntimes, nil
}

// Node lookup used in sanity checks.
type sanityCheckNodeLookup struct {
	nodes           map[signature.PublicKey]*node.Node
	nodesCertHashes map[hash.Hash]*node.Node

	nodesList []*node.Node
}

func (n *sanityCheckNodeLookup) NodeByConsensusOrP2PKey(key signature.PublicKey) (*node.Node, error) {
	node, ok := n.nodes[key]
	if !ok {
		return nil, ErrNoSuchNode
	}
	return node, nil
}

func (n *sanityCheckNodeLookup) NodeByCertificate(cert []byte) (*node.Node, error) {
	var h = hash.Hash{}
	h.FromBytes(cert)

	node, ok := n.nodesCertHashes[h]
	if !ok {
		return nil, ErrNoSuchNode
	}
	return node, nil
}

func (n *sanityCheckNodeLookup) Nodes() ([]*node.Node, error) {
	return n.nodesList, nil
}

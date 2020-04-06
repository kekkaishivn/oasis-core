// Package consensus provides the implementation agnostic consensus
// backend.
package api

import (
	"context"

	beacon "github.com/oasislabs/oasis-core/go/beacon/api"
	"github.com/oasislabs/oasis-core/go/common/cbor"
	"github.com/oasislabs/oasis-core/go/common/crypto/signature"
	"github.com/oasislabs/oasis-core/go/common/errors"
	"github.com/oasislabs/oasis-core/go/common/node"
	"github.com/oasislabs/oasis-core/go/common/pubsub"
	"github.com/oasislabs/oasis-core/go/consensus/api/transaction"
	epochtime "github.com/oasislabs/oasis-core/go/epochtime/api"
	genesis "github.com/oasislabs/oasis-core/go/genesis/api"
	keymanager "github.com/oasislabs/oasis-core/go/keymanager/api"
	registry "github.com/oasislabs/oasis-core/go/registry/api"
	roothash "github.com/oasislabs/oasis-core/go/roothash/api"
	scheduler "github.com/oasislabs/oasis-core/go/scheduler/api"
	staking "github.com/oasislabs/oasis-core/go/staking/api"
)

const (
	// moduleName is the module name used for error definitions.
	moduleName = "consensus"

	// HeightLatest is the height that represents the most recent block height.
	HeightLatest int64 = 0

	// VotingPower is the default voting power for all validator nodes.
	VotingPower = 1
)

var (
	// ErrNoCommittedBlocks is the error returned when there are no committed
	// blocks and as such no state can be queried.
	ErrNoCommittedBlocks = errors.New(moduleName, 1, "consensus: no committed blocks")

	// ErrOversizedTx is the error returned when the given transaction is too big to be processed.
	ErrOversizedTx = errors.New(moduleName, 2, "consensus: oversized transaction")

	// ErrVersionNotFound is the error returned when the given version (height) cannot be found,
	// possibly because it was pruned.
	ErrVersionNotFound = errors.New(moduleName, 3, "consensus: version not found")
)

// ClientBackend is a limited consensus interface used by clients that
// connect to the local node.
type ClientBackend interface {
	TransactionAuthHandler

	// SubmitTx submits a signed consensus transaction.
	SubmitTx(ctx context.Context, tx *transaction.SignedTransaction) error

	// StateToGenesis returns the genesis state at the specified block height.
	StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error)

	// EstimateGas calculates the amount of gas required to execute the given transaction.
	EstimateGas(ctx context.Context, req *EstimateGasRequest) (transaction.Gas, error)

	// WaitEpoch waits for consensus to reach an epoch.
	//
	// Note that an epoch is considered reached even if any epoch greater than
	// the one specified is reached (e.g., that the current epoch is already
	// in the future).
	WaitEpoch(ctx context.Context, epoch epochtime.EpochTime) error

	// GetEpoch returns the current epoch.
	GetEpoch(ctx context.Context, height int64) (epochtime.EpochTime, error)

	// GetBlock returns a consensus block at a specific height.
	GetBlock(ctx context.Context, height int64) (*Block, error)

	// GetTransactions returns a list of all transactions contained within a
	// consensus block at a specific height.
	//
	// NOTE: Any of these transactions could be invalid.
	GetTransactions(ctx context.Context, height int64) ([][]byte, error)

	// WatchBlocks returns a channel that produces a stream of consensus
	// blocks as they are being finalized.
	WatchBlocks(ctx context.Context) (<-chan *Block, pubsub.ClosableSubscription, error)
}

// Block is a consensus block.
//
// While some common fields are provided, most of the structure is dependent on
// the actual backend implementation.
type Block struct {
	// Height contains the block height.
	Height int64 `json:"height"`
	// Meta contains the consensus backend specific block metadata.
	Meta cbor.RawMessage `json:"meta"`
}

// Backend is an interface that a consensus backend must provide.
type Backend interface {
	ClientBackend

	// Synced returns a channel that is closed once synchronization is
	// complete.
	Synced() <-chan struct{}

	// ConsensusKey returns the consensus signing key.
	ConsensusKey() signature.PublicKey

	// GetAddresses returns the consensus backend addresses.
	GetAddresses() ([]node.ConsensusAddress, error)

	// RegisterGenesisHook registers a function to be called when the
	// consensus backend is initialized from genesis (e.g., on fresh
	// start).
	//
	// Note that these hooks block consensus genesis from completing
	// while they are running.
	RegisterGenesisHook(func())

	// RegisterHaltHook registers a function to be called when the
	// consensus Halt epoch height is reached.
	RegisterHaltHook(func(ctx context.Context, blockHeight int64, epoch epochtime.EpochTime))

	// SubmitEvidence submits evidence of misbehavior.
	SubmitEvidence(ctx context.Context, evidence Evidence) error

	// SubmissionManager returns the transaction submission manager.
	SubmissionManager() SubmissionManager

	// EpochTime returns the epochtime backend.
	EpochTime() epochtime.Backend

	// Beacon returns the beacon backend.
	Beacon() beacon.Backend

	// KeyManager returns the keymanager backend.
	KeyManager() keymanager.Backend

	// Registry returns the registry backend.
	Registry() registry.Backend

	// RootHash returns the roothash backend.
	RootHash() roothash.Backend

	// Staking returns the staking backend.
	Staking() staking.Backend

	// Scheduler returns the scheduler backend.
	Scheduler() scheduler.Backend
}

// TransactionAuthHandler is the interface for handling transaction authentication
// (checking nonces and fees).
type TransactionAuthHandler interface {
	// GetSignerNonce returns the nonce that should be used by the given
	// signer for transmitting the next transaction.
	GetSignerNonce(ctx context.Context, req *GetSignerNonceRequest) (uint64, error)
}

// EvidenceKind is kind of evindence of a node misbehaving.
type EvidenceKind int

const (
	// EvidenceKindConsensus is consensus-layer specific evidence.
	EvidenceKindConsensus EvidenceKind = 0

	EvidenceKindMax = EvidenceKindConsensus
)

// String returns a string representation of an EvidenceKind.
func (k EvidenceKind) String() string {
	switch k {
	case EvidenceKindConsensus:
		return "consensus"
	default:
		return "[unknown evidence kind]"
	}
}

// Evidence is evidence of a node misbehaving.
type Evidence interface {
	// Kind returns the evidence kind.
	Kind() EvidenceKind
	// Unwrap returns the unwrapped evidence (if any).
	Unwrap() interface{}
}

// ConsensusEvidence is consensus backend-specific evidence.
type ConsensusEvidence struct {
	inner interface{}
}

var _ Evidence = (*ConsensusEvidence)(nil)

// Kind returns the evidence kind.
func (ce ConsensusEvidence) Kind() EvidenceKind {
	return EvidenceKindConsensus
}

// Unwrap returns the unwrapped evidence (if any).
func (ce ConsensusEvidence) Unwrap() interface{} {
	return ce.inner
}

// NewConsensusEvidence creates new consensus backend-specific evidence.
func NewConsensusEvidence(inner interface{}) ConsensusEvidence {
	return ConsensusEvidence{inner: inner}
}

// EstimateGasRequest is a EstimateGas request.
type EstimateGasRequest struct {
	Caller      signature.PublicKey      `json:"caller"`
	Transaction *transaction.Transaction `json:"transaction"`
}

// GetSignerNonceRequest is a GetSignerNonce request.
type GetSignerNonceRequest struct {
	ID     signature.PublicKey `json:"id"`
	Height int64               `json:"height"`
}

package roothash

import (
	"errors"
	"time"

	"github.com/oasislabs/oasis-core/go/common/cbor"
	"github.com/oasislabs/oasis-core/go/roothash/api/block"
	"github.com/oasislabs/oasis-core/go/roothash/api/commitment"
)

var (
	_ cbor.Marshaler   = (*round)(nil)
	_ cbor.Unmarshaler = (*round)(nil)
)

type round struct {
	ComputePool *commitment.MultiPool `json:"compute_pool"`
	MergePool   *commitment.Pool      `json:"merge_pool"`

	CurrentBlock *block.Block `json:"current_block"`
	Finalized    bool         `json:"finalized"`
}

func (r *round) reset() {
	r.ComputePool.ResetCommitments()
	r.MergePool.ResetCommitments()
	r.Finalized = false
}

func (r *round) getNextTimeout() (timeout time.Time) {
	timeout = r.ComputePool.GetNextTimeout()
	if timeout.IsZero() || (!r.MergePool.NextTimeout.IsZero() && r.MergePool.NextTimeout.Before(timeout)) {
		timeout = r.MergePool.NextTimeout
	}
	return
}

func (r *round) addComputeCommitment(commitment *commitment.ComputeCommitment, sv commitment.SignatureVerifier) (*commitment.Pool, error) {
	if r.Finalized {
		return nil, errors.New("tendermint/roothash: round is already finalized, can't commit")
	}
	return r.ComputePool.AddComputeCommitment(r.CurrentBlock, sv, commitment)
}

func (r *round) addMergeCommitment(commitment *commitment.MergeCommitment, sv commitment.SignatureVerifier) error {
	if r.Finalized {
		return errors.New("tendermint/roothash: round is already finalized, can't commit")
	}
	return r.MergePool.AddMergeCommitment(r.CurrentBlock, sv, commitment, r.ComputePool)
}

func (r *round) transition(blk *block.Block) {
	r.CurrentBlock = blk
	r.reset()
}

// MarshalCBOR serializes the type into a CBOR byte vector.
func (r *round) MarshalCBOR() []byte {
	return cbor.Marshal(r)
}

// UnmarshalCBOR deserializes a CBOR byte vector into given type.
func (r *round) UnmarshalCBOR(data []byte) error {
	return cbor.Unmarshal(data, r)
}

func newRound(
	computePool *commitment.MultiPool,
	mergePool *commitment.Pool,
	blk *block.Block,
) *round {
	r := &round{
		CurrentBlock: blk,
		ComputePool:  computePool,
		MergePool:    mergePool,
	}
	r.reset()

	return r
}

package abci

import (
	"context"
	"fmt"
	"strings"

	"github.com/oasislabs/oasis-core/go/common/logging"
	nodedb "github.com/oasislabs/oasis-core/go/storage/mkvs/db/api"
)

const (
	// PruneDefault is the default PruneStrategy.
	PruneDefault = pruneNone

	pruneNone  = "none"
	pruneKeepN = "keep_n"
)

// PruneStrategy is the strategy to use when pruning the ABCI mux iAVL
// state.
type PruneStrategy int

const (
	// PruneNone retains all versions.
	PruneNone PruneStrategy = iota

	// PruneKeepN retains the last N latest versions.
	PruneKeepN
)

func (s PruneStrategy) String() string {
	switch s {
	case PruneNone:
		return pruneNone
	case PruneKeepN:
		return pruneKeepN
	default:
		return "[unknown]"
	}
}

func (s *PruneStrategy) FromString(str string) error {
	switch strings.ToLower(str) {
	case pruneNone:
		*s = PruneNone
	case pruneKeepN:
		*s = PruneKeepN
	default:
		return fmt.Errorf("abci/pruner: unknown pruning strategy: '%v'", str)
	}

	return nil
}

// PruneConfig is the pruning strategy and related configuration.
type PruneConfig struct {
	// Strategy is the PruneStrategy used.
	Strategy PruneStrategy

	// NumKept is the number of versions retained when applicable.
	NumKept uint64
}

// StatePruner is a concrete ABCI mux state pruner implementation.
type StatePruner interface {
	// Prune purges unneeded versions from the ABCI mux node database,
	// given the latest version, based on the underlying strategy.
	Prune(ctx context.Context, latestVersion uint64) error
}

type statePrunerInitializer interface {
	Initialize(latestVersion uint64) error
}

type nonePruner struct{}

func (p *nonePruner) Prune(ctx context.Context, latestVersion uint64) error {
	// Nothing to prune.
	return nil
}

type genericPruner struct {
	logger *logging.Logger
	ndb    nodedb.NodeDB

	earliestVersion uint64
	keepN           uint64
}

func (p *genericPruner) Initialize(latestVersion uint64) error {
	// Figure out the eldest version currently present in the tree.
	var err error
	if p.earliestVersion, err = p.ndb.GetEarliestVersion(context.Background()); err != nil {
		return fmt.Errorf("failed to get earliest version: %w", err)
	}

	return p.doPrune(context.Background(), latestVersion)
}

func (p *genericPruner) Prune(ctx context.Context, latestVersion uint64) error {
	if err := p.doPrune(ctx, latestVersion); err != nil {
		p.logger.Error("Prune",
			"err", err,
		)
		return err
	}
	return nil
}

func (p *genericPruner) doPrune(ctx context.Context, latestVersion uint64) error {
	if latestVersion < p.keepN {
		return nil
	}

	p.logger.Debug("Prune: Start",
		"latest_version", latestVersion,
		"start_version", p.earliestVersion,
	)

	preserveFrom := latestVersion - p.keepN
	for i := p.earliestVersion; i <= latestVersion; i++ {
		if i >= preserveFrom {
			p.earliestVersion = i
			break
		}

		p.logger.Debug("Prune: Delete",
			"latest_version", latestVersion,
			"pruned_version", i,
		)

		err := p.ndb.Prune(ctx, i)
		switch err {
		case nil:
		case nodedb.ErrNotEarliest:
			p.logger.Debug("Prune: skipping non-earliest version",
				"version", i,
			)
			continue
		default:
			return err
		}
	}

	p.logger.Debug("Prune: Finish",
		"latest_version", latestVersion,
		"eldest_version", p.earliestVersion,
	)

	return nil
}

func newStatePruner(cfg *PruneConfig, ndb nodedb.NodeDB, latestVersion uint64) (StatePruner, error) {
	// The roothash checkCommittees call requires at least 1 previous block
	// for timekeeping purposes.
	const minKept = 1

	logger := logging.GetLogger("abci-mux/pruner")

	var statePruner StatePruner
	switch cfg.Strategy {
	case PruneNone:
		statePruner = &nonePruner{}
	case PruneKeepN:
		if cfg.NumKept < minKept {
			return nil, fmt.Errorf("abci/pruner: invalid number of versions retained: %v", cfg.NumKept)
		}

		statePruner = &genericPruner{
			logger: logger,
			ndb:    ndb,
			keepN:  cfg.NumKept,
		}
	default:
		return nil, fmt.Errorf("abci/pruner: unsupported pruning strategy: %v", cfg.Strategy)
	}

	if initializer, ok := statePruner.(statePrunerInitializer); ok {
		if err := initializer.Initialize(latestVersion); err != nil {
			return nil, err
		}
	}

	logger.Debug("ABCI state pruner initialized",
		"strategy", cfg.Strategy,
		"num_kept", cfg.NumKept,
	)

	return statePruner, nil
}

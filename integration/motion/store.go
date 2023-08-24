package motion

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/motion/blob"
	"github.com/filecoin-shipyard/jiffy"
	"github.com/filecoin-shipyard/telefil"
	"github.com/ipfs/go-log/v2"
)

var (
	logger            = log.Logger("jiffy/integration/motion")
	_      blob.Store = (*Store)(nil)
)

type (
	Store struct {
		// TODO implement persistent storage
		blobMeta map[string]abi.PieceInfo
		fil      *telefil.Telefil
		j        *jiffy.Jiffy
	}
)

// NewStore instantiates a Motion blob.Store backed by jiffy.Jiffy.
// The instantiated Store must be started before use, and shut down when no longer in use.
// See: Store.Start, Store.Shutdown.
func NewStore(o ...Option) (blob.Store, error) {
	opts, err := newOptions(o...)
	if err != nil {
		logger.Errorw("failed to parse store options", "err", err)
		return nil, err
	}
	var store Store
	if store.fil, err = telefil.New(opts.telefilOptions...); err != nil {
		logger.Errorw("failed to instantiate telefil Filecoin client", "err", err)
		return nil, err
	}
	if store.j, err = jiffy.New(opts.jiffyOptions...); err != nil {
		logger.Errorw("failed to instantiate jiffy backing store", "err", err)
		return nil, err
	}
	return &store, nil
}

func (s *Store) Start(ctx context.Context) error {
	if err := s.j.Start(ctx); err != nil {
		logger.Errorw("failed to start jiffy backing store", "err", err)
		return err
	}
	logger.Info("jiffy motion store started")
	return nil
}

func (s *Store) Put(ctx context.Context, in io.ReadCloser) (*blob.Descriptor, error) {
	id, err := blob.NewID()
	if err != nil {
		return nil, err
	}
	// TODO check max size
	// TODO implement support for multiple segments to handle larger blobs.
	switch segment, err := s.j.Segment(ctx, in); {
	case errors.Is(err, jiffy.ErrSegmentTooLarge):
		logger.Error("blob exceeds the maximum allowed segment size")
		return nil, blob.ErrBlobTooLarge
	case err != nil:
		logger.Errorw("failed to segment blob", "err", err)
		return nil, err
	default:
		s.blobMeta[id.String()] = segment.Info
		logger.Debugw("blob stored successfully", "id", id, "segmentInfo", segment.Info)
		return &blob.Descriptor{
			ID:               *id,
			Size:             segment.RawSize,
			ModificationTime: segment.CreateTime, // TODO: change this to create time in motion? since blobs are immutable.
		}, nil
	}
}

func (s *Store) Describe(ctx context.Context, id blob.ID) (*blob.Descriptor, error) {
	info, ok := s.blobMeta[id.String()]
	if !ok {
		return nil, blob.ErrBlobNotFound
	}

	segment, err := s.j.GetSegment(ctx, info)
	switch {
	case errors.Is(err, jiffy.ErrSegmentNotFound):
		// Store must be corrupt or there is data loss.
		// TODO think if we can do more here.
		logger.Errorw("segment not found for blob", "id", id, "segmentInfo", info)
		return nil, blob.ErrBlobNotFound
	}

	// Get genesis blocks and chain head first to fail early if there is an issue getting them.
	genesis, err := s.fil.ChainGetGenesis(ctx)
	if err != nil {
		logger.Errorw("failed to get chain genesis block, required to calculate replica expiration time", "err", err)
		return nil, err
	} else if len(genesis.Blocks) < 1 {
		logger.Errorw("chain has no genesis blocks", "genesis", genesis)
		return nil, errors.New("no genesis blocks")
	}
	genesisTime := time.Unix(genesis.Blocks[0].Timestamp, 0)
	head, err := s.fil.ChainHead(ctx) // TODO cache, look back with tipset.
	if err != nil {
		logger.Errorw("failed to get chain height, required to determine the replica status", "err", err)
		return nil, err
	}

	// Get the replicas from jiffy replicator
	replicas, err := s.j.GetReplicas(ctx, info)
	if err != nil {
		return nil, err
	}

	desc := blob.Descriptor{
		ID:               id,
		Size:             segment.RawSize,
		ModificationTime: segment.CreateTime,
	}
	if len(replicas) == 0 {
		logger.Debugw("no replicas found for blob", "id", id)
		return &desc, nil
	}
	desc.Status = &blob.Status{
		Replicas: make([]blob.Replica, 0, len(replicas)),
	}
	for _, replica := range replicas {
		desc.Status.Replicas = append(desc.Status.Replicas, blob.Replica{
			Provider: replica.Provider().String(),
			// TODO Status returned by jiffy is richer than the permutations Motion allows; see:
			//       - https://github.com/filecoin-project/motion/blob/a06961a8361406a44bb423e86223262b6c4af493/openapi.yaml#L112
			//      Either update motion or reduce resolution by bespoke mapping here.
			Status:       replica.Status(head.Height).String(),
			LastVerified: replica.LastChecked,
			Expiration:   replica.Expiration(genesisTime),
		})
	}
	logger.Debugw("replicas found for blob", "id", id, "count", len(replicas))
	return &desc, nil
}

func (s *Store) Get(ctx context.Context, id blob.ID) (io.ReadSeekCloser, error) {
	info, ok := s.blobMeta[id.String()]
	if !ok {
		return nil, blob.ErrBlobNotFound
	}
	// TODO we need to strip off CAR section extras from the reader to match original
	return s.j.Retrieve(ctx, info)
}

func (s *Store) Shutdown(ctx context.Context) error {
	if err := s.j.Shutdown(ctx); err != nil {
		logger.Errorw("failed to shut down jiffy backing store", "err", err)
		return err
	}
	logger.Info("jiffy motion store stopped")
	return nil
}

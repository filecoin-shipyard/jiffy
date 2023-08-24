package jiffy

import (
	"context"
	"io"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/ipfs/go-log/v2"
)

var (
	logger = log.Logger("jiffy")

	_ Segmentor  = (*Jiffy)(nil)
	_ Replicator = (*Jiffy)(nil)
	_ Retriever  = (*Jiffy)(nil)
)

type (
	Jiffy struct {
		*options

		offloader  Offloader
		segmentor  Segmentor
		replicator Replicator
		retriever  Retriever
		dealer     Dealer
	}
)

func New(o ...Option) (*Jiffy, error) {
	var err error
	var j Jiffy
	if j.options, err = newOptions(o...); err != nil {
		return nil, err
	}
	if s, err := newHeadlessCarSegmentor(&j); err != nil {
		return nil, err
	} else {
		j.segmentor = s
		// Use the headless segmentor as retriever until we implement remote HTTP piece retrieval with local fallback.
		// Using the segmentor in this way essentially means we get local retrieval only.
		// TODO replace with remote retriever with local retrieval fallback.
		j.retriever = s
	}
	if j.replicator, err = newSimpleReplicator(&j); err != nil {
		return nil, err
	}
	if j.dealer, err = newStorageMarketDealer_1_2_0(&j); err != nil {
		return nil, err
	}
	if j.offloader, err = newHttpOffloader(&j); err != nil {
		return nil, err
	}
	return &j, nil
}

func (j *Jiffy) Start(ctx context.Context) error {
	type starter interface {
		Start(ctx context.Context) error
	}
	for _, component := range []any{j.segmentor, j.replicator, j.replicator, j.offloader} {
		if svc, ok := component.(starter); ok {
			if err := svc.Start(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (j *Jiffy) Retrieve(ctx context.Context, info abi.PieceInfo) (io.ReadSeekCloser, error) {
	return j.retriever.Retrieve(ctx, info)
}

func (j *Jiffy) GetReplicas(ctx context.Context, info abi.PieceInfo) ([]Replica, error) {
	return j.replicator.GetReplicas(ctx, info)
}

func (j *Jiffy) Segment(ctx context.Context, closer io.ReadCloser) (*Segment, error) {
	return j.segmentor.Segment(ctx, closer)
}

func (j *Jiffy) GetSegment(ctx context.Context, info abi.PieceInfo) (*Segment, error) {
	return j.segmentor.GetSegment(ctx, info)
}

func (j *Jiffy) ListSegments(ctx context.Context) ([]*Segment, error) {
	return j.segmentor.ListSegments(ctx)
}

func (j *Jiffy) Shutdown(ctx context.Context) error {
	type shutdowner interface {
		Shutdown(ctx context.Context) error
	}
	var err error // TODO use multierr
	for _, component := range []any{j.segmentor, j.replicator, j.replicator, j.offloader} {
		if svc, ok := component.(shutdowner); ok {
			err = svc.Shutdown(ctx)
		}
	}
	return err
}

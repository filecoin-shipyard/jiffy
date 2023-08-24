package jiffy

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-shipyard/jiffy/car"
	"github.com/ipfs/go-cid"
	chunk "github.com/ipfs/go-ipfs-chunker"
)

var (
	_ Segmentor = (*headlessCarSegmentor)(nil)
	_ Retriever = (*headlessCarSegmentor)(nil)
)

type (
	Segmentor interface {
		Segment(context.Context, io.ReadCloser) (*Segment, error)
		GetSegment(context.Context, abi.PieceInfo) (*Segment, error)
		ListSegments(context.Context) ([]*Segment, error)
	}
	Segment struct {
		Info abi.PieceInfo
		// Size returns the original data size
		RawSize uint64
		// SegmentedSize is the CAR-ified size.
		SegmentedSize uint64
		// CreateTime is the time at which this segment was created.
		CreateTime time.Time
	}

	headlessCarSegmentor struct {
		j        *Jiffy
		segments map[cid.Cid]headlessCarSegment // TODO implement persistent storage
	}
	headlessCarSegment struct {
		Segment
		path string // TODO might not need this
	}
)

// TODO: Implement unixfs variation of segmentor.
//       We need unixfs size pre-calculation to use unixfs DAG format here.
//       See: https://github.com/ipfs/go-unixfsnode/issues/58

func newHeadlessCarSegmentor(j *Jiffy) (*headlessCarSegmentor, error) {
	return &headlessCarSegmentor{j: j}, nil
}

func (c *headlessCarSegmentor) Segment(ctx context.Context, in io.ReadCloser) (*Segment, error) {

	// TODO: because segmentation in Jiffy works with piece CIDs, we can detect duplicate blobs. Capitalize on it.

	sf, err := os.CreateTemp(c.j.segmentorStoreDir, "*.temp")
	if err != nil {
		return nil, err
	}
	splitter := chunk.NewSizeSplitter(in, c.j.segmentorChunkSizeBytes)
	var cc commp.Calc
	var rawSize, segmentedSize uint64
	out := io.MultiWriter(&cc, sf)
	var erroneousCleanup = func() {
		_ = sf.Close()
		cc.Reset()
		_ = os.Remove(sf.Name()) // TODO revisit in support for resumption.
	}
	for {
		select {
		case <-ctx.Done():
			erroneousCleanup()
			return nil, ctx.Err()
		default:
		}
		switch b, err := splitter.NextBytes(); {
		case errors.Is(err, io.EOF):
			// FIXME check for data being too small; we must write at least 65 bytes.
			p, pieceSize, err := cc.Digest()
			if err != nil {
				erroneousCleanup()
				return nil, err
			}
			pcid, err := commcid.PieceCommitmentV1ToCID(p)
			if err != nil {
				erroneousCleanup()
				return nil, err
			}
			_ = sf.Close()
			finalSegmentPath := filepath.Join(c.j.segmentorStoreDir, pcid.String()+".headless.car")
			if err := os.Rename(sf.Name(), finalSegmentPath); err != nil {
				// TODO check for already exists error meaning blob is duplicate
				return nil, err
			}
			segment := headlessCarSegment{
				Segment: Segment{
					Info: abi.PieceInfo{
						Size:     abi.PaddedPieceSize(pieceSize),
						PieceCID: pcid,
					},
					RawSize:       rawSize,
					SegmentedSize: segmentedSize,
				},
				path: finalSegmentPath,
			}
			c.segments[pcid] = segment
			return &segment.Segment, nil
		case err != nil:
			erroneousCleanup()
			return nil, err
		default:
			rawSize += uint64(len(b))
			if rawSize > uint64(c.j.segmentorMaxTotalSizeBytes) {
				erroneousCleanup()
				return nil, ErrSegmentTooLarge
			}
			sectionSize, err := car.Section(b).WriteTo(out)
			if err != nil {
				erroneousCleanup()
				return nil, err
			}
			segmentedSize += uint64(sectionSize)
		}
	}
}

func (c *headlessCarSegmentor) GetSegment(ctx context.Context, info abi.PieceInfo) (*Segment, error) {
	segment, ok := c.segments[info.PieceCID]
	if !ok {
		return nil, ErrSegmentNotFound
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		return &segment.Segment, nil
	}
}

func (c *headlessCarSegmentor) ListSegments(ctx context.Context) ([]*Segment, error) {
	switch count := len(c.segments); {
	case count == 0:
		return nil, nil
	default:
		list := make([]*Segment, 0, count)
		for _, segment := range c.segments {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				list = append(list, &segment.Segment)
			}
		}
		return list, nil
	}
}

func (c *headlessCarSegmentor) Retrieve(ctx context.Context, info abi.PieceInfo) (io.ReadSeekCloser, error) {
	segment, ok := c.segments[info.PieceCID]
	if !ok {
		return nil, ErrSegmentNotFound
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// TODO: put safeguard in place to not delete the file if it is being read.
		// TODO differentiate between reading segments and raw data.
		//      For latter we need to strip off CAR section fluff.
		return os.Open(segment.path)
	}
}

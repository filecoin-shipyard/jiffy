package jiffy

import (
	"math/bits"
	"sort"

	"github.com/filecoin-project/go-commp-utils/nonffi"
	commcid "github.com/filecoin-project/go-fil-commcid"
	commp "github.com/filecoin-project/go-fil-commp-hashhash"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-shipyard/jiffy/car"
)

var (
	_ sort.Interface = (*Segments)(nil)

	emptyHeaderV1Segment *Segment
)

func init() {
	var cp commp.Calc
	// We don't have to use an empty CARv1 header. It can include CIDs if needed.
	// But it is best to keep its size below 128 bytes to reduce the amount of padding that's needed for fr32.
	written, err := cp.Write(car.EmptyHeaderV1Bytes)
	if err != nil {
		panic(err)
	}
	paddingLength := commp.MinPiecePayload - uint64(written)
	if paddingLength > 0 {
		if _, err := cp.Write(make([]byte, paddingLength)); err != nil {
			panic(err)
		}
	}
	p, size, err := cp.Digest()
	if err != nil {
		panic(err)
	}
	cid, err := commcid.PieceCommitmentV1ToCID(p)
	if err != nil {
		panic(err)
	}
	headerSize := uint64(len(car.EmptyHeaderV1Bytes))
	emptyHeaderV1Segment = &Segment{
		Info: abi.PieceInfo{
			Size:     abi.PaddedPieceSize(size),
			PieceCID: cid,
		},
		RawSize:       headerSize,
		SegmentedSize: headerSize,
	}
}

type (
	Piece struct {
		Info     abi.PieceInfo
		Capacity abi.PaddedPieceSize
		Segments Segments
		// TotalSegmentedSize is the sum of all Segment.SegmentedSize segments in this piece.
		TotalSegmentedSize uint64
	}
	Segments []*Segment
)

func (s Segments) Len() int           { return len(s) }
func (s Segments) Less(i, j int) bool { return s[i].Info.Size < s[j].Info.Size }
func (s Segments) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func NewPiece(capacity abi.PaddedPieceSize) *Piece {
	return &Piece{Capacity: capacity}
}

func (p *Piece) canAdd(segment *Segment) bool {
	return segment.Info.Size <= p.Capacity-p.Info.Size
}

func (p *Piece) addSegment(segment *Segment) {
	p.Segments = append(p.Segments, segment)
	p.Info.Size += segment.Info.Size
}

func packBestFit(segments []*Segment, pieceCapacity abi.PaddedPieceSize, maxPieces int) ([]*Piece, []*Segment, error) {
	pieceCapacity = pieceCapacity - emptyHeaderV1Segment.Info.Size
	sort.Sort(sort.Reverse(Segments(segments)))
	var pieces []*Piece
	var unpackedSegments []*Segment
	for _, segment := range segments {
		minRemainder := pieceCapacity
		bestPieceIndex := -1
		for pieceIndex, piece := range pieces {
			remaining := piece.Capacity - piece.Info.Size
			if piece.canAdd(segment) && remaining-segment.Info.Size < minRemainder {
				bestPieceIndex = pieceIndex
				minRemainder = remaining - segment.Info.Size
			}
		}
		if bestPieceIndex != -1 {
			pieces[bestPieceIndex].addSegment(segment)
			continue
		}
		if len(pieces) < maxPieces {
			newPiece := NewPiece(pieceCapacity)
			if newPiece.canAdd(segment) {
				newPiece.addSegment(segment)
				pieces = append(pieces, newPiece)
				continue
			}
		}
		unpackedSegments = append(unpackedSegments, segment)
	}

	// Finalize pieces, i.e. for each piece:
	// 1. sort pieces by segment piece size, to minimise the need for fr32 padding
	// 2. prepend an empty car header, to turn the aggregate data represented by the piece into a valid CARv1.
	// 3. calculate the aggregate piece CID and padded size.
	for _, p := range pieces {
		sort.Sort(p.Segments)
		// Prepend the empty CAR header as a segment, which should always be of minimum piece payload size
		p.Segments = append([]*Segment{emptyHeaderV1Segment}, p.Segments...)
		p.Info.Size += emptyHeaderV1Segment.Info.Size
		segmentInfos := make([]abi.PieceInfo, len(p.Segments))
		for i, segment := range p.Segments {
			segmentInfos[i] = segment.Info
		}
		var err error
		if p.Info.PieceCID, err = nonffi.GenerateUnsealedCID(abi.RegisteredSealProof_StackedDrg64GiBV1, segmentInfos); err != nil {
			return nil, nil, err
		}
		// Check if total piece size is valid, i.e. is a power of 2
		if bits.OnesCount64(uint64(p.Info.Size)) != 1 {
			// Find the next largest power of 2 number
			p.Info.Size = 1 << uint64(bits.Len64(uint64(p.Info.Size)))
		}
	}
	return pieces, unpackedSegments, nil
}

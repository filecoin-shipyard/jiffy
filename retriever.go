package jiffy

import (
	"context"
	"io"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-shipyard/boostly"
)

var (
	_ Retriever = (*httpPieceRetriever)(nil)
)

type (
	Retriever interface {
		Retrieve(context.Context, abi.PieceInfo) (io.ReadSeekCloser, error)
	}
	httpPieceRetriever struct {
		j *Jiffy
	}
)

// TODO implement chained retriever where it returns things in this order:
//       1. well-known pieces like the empty carv1 header segment
//       2. local copy (via headless car segmentor)
//       3. remote HTTP piece retrieval by piece CID.
//     Note for other protocols we need CIDs from the CAR sections.
//     This basically means we either store segments as unixfs file and keep hold of the root
//     or store the CARv2 index of the CAR sections.
//     Other techniques include nested CAR in sections which has its own headaches.
//     For now, require SPs to support HTTP piece retrieval.

//lint:ignore U1000 WIP
func newHttpPieceRetriever(j *Jiffy) (*httpPieceRetriever, error) {
	return &httpPieceRetriever{j: j}, nil
}

func (s *httpPieceRetriever) Retrieve(ctx context.Context, pi abi.PieceInfo) (io.ReadSeekCloser, error) {
	// TODO add built-in rule for well known piece infos like emptyHeaderV1Segment
	replicas, err := s.j.replicator.GetReplicas(ctx, pi)
	if err != nil {
		return nil, err
	}
	if len(replicas) == 0 {
		return nil, nil
	}

	// Pick a replica as candidate to retrieve from
	for range replicas {
		// TODO for each replica check status and pick an active one.
	}

	var sp address.Address
	info, err := s.j.fil.StateMinerInfo(ctx, sp)
	if err != nil {
		return nil, err
	}
	if err = s.j.h.Connect(ctx, *info); err != nil {
		return nil, err
	}

	// Check what transports the SP supports
	transports, err := boostly.QueryTransports(ctx, s.j.h, info.ID)
	if err != nil {
		return nil, err
	}
	for _, protocol := range transports.Protocols {
		//lint:ignore SA9003 WIP
		if protocol.Name == "http" {
			// might support /ipfs trustless gateway
			// might suport /piece
		}
	}
	// TODO implement the rest of remote retrieval
	return nil, nil
}

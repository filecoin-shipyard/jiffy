package jiffy

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/filecoin-project/go-address"
	cborutil "github.com/filecoin-project/go-cbor-util"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/go-state-types/builtin/v9/market"
	"github.com/filecoin-shipyard/boostly"
	"github.com/google/uuid"
)

const (
	epochsInMinDeal = builtin.EpochsInDay * 6 * 30
)

var (
	_ Dealer = (*storageMarketDealer_1_2_0)(nil)
)

type (
	Dealer interface {
		Deal(context.Context, *Piece, address.Address) (*boostly.DealProposal, error)
	}
	storageMarketDealer_1_2_0 struct {
		j *Jiffy
	}
)

func newStorageMarketDealer_1_2_0(j *Jiffy) (*storageMarketDealer_1_2_0, error) {
	return &storageMarketDealer_1_2_0{j: j}, nil
}

func (d *storageMarketDealer_1_2_0) Deal(ctx context.Context, piece *Piece, sp address.Address) (*boostly.DealProposal, error) {
	client, err := d.j.wallet.Address()
	if err != nil {
		return nil, err
	}
	label, err := market.NewLabelFromString(piece.Info.PieceCID.String()) // TODO what would make a good label?
	if err != nil {
		return nil, err
	}
	spAddr, err := d.j.fil.StateMinerInfo(ctx, sp) // Cache sp -> peer.AddrInfo or maybe integrate with HeyFil
	if err != nil {
		return nil, err
	}
	if err := d.j.h.Connect(ctx, *spAddr); err != nil {
		return nil, err
	}
	protocol, err := d.j.h.Peerstore().FirstSupportedProtocol(spAddr.ID, boostly.FilStorageMarketProtocol_1_2_0)
	if err != nil {
		return nil, err
	}
	if protocol == "" {
		return nil, fmt.Errorf("sp %s does not support %s", sp.String(), boostly.FilStorageMarketProtocol_1_2_0)
	}
	head, err := d.j.fil.ChainHead(ctx)
	if err != nil {
		return nil, err
	}
	start := head.Height + d.j.dealStartDelay
	end := start + d.j.dealDuration
	if epochsInDeal := end - start; epochsInDeal < epochsInMinDeal {
		return nil, fmt.Errorf("deal duration must be at least 6 moths in epochs (%d), got: %d", epochsInMinDeal, epochsInDeal)
	}
	// TODO check max for network v1 or should we bother?

	bounds, err := d.j.fil.StateDealProviderCollateralBounds(ctx, piece.Info.Size, d.j.dealVerified)
	if err != nil {
		return nil, err
	}
	collateral := d.j.dealProviderCollateralPicker(bounds.Min, bounds.Max)
	price := d.j.dealPricePerEpochPicker(piece.Info.Size, start, end)
	mp := market.DealProposal{
		PieceCID:             piece.Info.PieceCID,
		PieceSize:            piece.Info.Size,
		VerifiedDeal:         d.j.dealVerified,
		Client:               client,
		Provider:             sp,
		Label:                label,
		StartEpoch:           start,
		EndEpoch:             end,
		StoragePricePerEpoch: price,
		ProviderCollateral:   collateral,
	}
	mpb, err := cborutil.Dump(mp)
	if err != nil {
		return nil, err
	}
	signature, err := d.j.wallet.Sign(ctx, mpb)
	if err != nil {
		return nil, err
	}

	offload, err := d.j.offloader.Offload(piece)
	if err != nil {
		return nil, err
	}
	params, err := json.Marshal(boostly.HttpRequest{
		URL:     offload.URL.String(),
		Headers: offload.Headers,
	})
	if err != nil {
		return nil, err
	}

	dealUuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	proposal := boostly.DealProposal{
		DealUUID:  dealUuid,
		IsOffline: d.j.options.dealOffline,
		ClientDealProposal: market.ClientDealProposal{
			Proposal:        mp,
			ClientSignature: *signature,
		},
		// TODO: data root means nothing in the context of jiffy; rivisit for unixfs CAR files.
		DealDataRoot: piece.Info.PieceCID,

		// Boost ignores transfer if IsOffline is set to true.
		// Regardless, set the transfer as documentation of how the SP is going to get the data.
		// TODO: Discuss options to read this with Boost team if set for offline deals.
		Transfer: boostly.Transfer{
			Type:   offload.Type,
			Params: params,
			Size:   piece.TotalSegmentedSize,
		},
		RemoveUnsealedCopy: d.j.dealRemoveUnsealedCopy,
		SkipIPNIAnnounce:   d.j.dealSkipIPNIAnnounce,
	}

	resp, err := boostly.ProposeDeal(ctx, d.j.h, spAddr.ID, proposal)
	if err != nil {
		return nil, err
	}
	if !resp.Accepted {
		return nil, fmt.Errorf("deal was not accepted by %s: %s", sp, resp.Message)
	}
	return &proposal, nil
}

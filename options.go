package jiffy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-shipyard/telefil"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
)

type (
	// Option represents a configurable parameter in Motion service.
	Option  func(*options) error
	options struct {
		h      host.Host
		fil    *telefil.Telefil
		wallet Wallet

		replicatorSpPicker             func(context.Context, *Piece) ([]address.Address, error)
		replicatorInterval             *time.Ticker
		replicatorVerificationInterval *time.Ticker

		segmentorStoreDir          string
		segmentorChunkSizeBytes    int64
		segmentorMaxTotalSizeBytes int64

		dealProviderCollateralPicker func(min, max abi.TokenAmount) abi.TokenAmount
		dealPricePerEpochPicker      func(pieceSize abi.PaddedPieceSize, start, end abi.ChainEpoch) abi.TokenAmount
		dealVerified                 bool
		dealSkipIPNIAnnounce         bool
		dealRemoveUnsealedCopy       bool
		dealOffline                  bool
		dealPricePerGiBEpoch         abi.TokenAmount
		dealPricePerGiB              abi.TokenAmount
		dealPricePerDeal             abi.TokenAmount
		dealStartDelay               abi.ChainEpoch
		dealDuration                 abi.ChainEpoch
	}
)

func newOptions(o ...Option) (*options, error) {
	opts := options{
		dealProviderCollateralPicker: func(min, max abi.TokenAmount) abi.TokenAmount { return min },
		dealStartDelay:               builtin.EpochsInDay * 4,
		dealDuration:                 builtin.EpochsInYear,
		dealOffline:                  true,
		segmentorMaxTotalSizeBytes:   31 * GiB,
		segmentorChunkSizeBytes:      1 * MiB,
		dealVerified:                 true,

		replicatorInterval:             time.NewTicker(1 * time.Hour),
		replicatorVerificationInterval: time.NewTicker(1 * time.Hour),
	}
	for _, apply := range o {
		if err := apply(&opts); err != nil {
			return nil, err
		}
	}
	if opts.wallet == nil {
		return nil, errors.New("wallet must be specified")
	}
	if opts.h == nil {
		var err error
		if opts.h, err = libp2p.New(); err != nil {
			return nil, err
		}
	}
	if opts.segmentorStoreDir == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		opts.segmentorStoreDir = filepath.Join(userHome, ".jiffy", "segments")
	}
	if opts.replicatorSpPicker == nil {
		return nil, fmt.Errorf("storage provider picker must be set or at least one storage provider must be configured")
	}
	if opts.fil == nil {
		var err error
		if opts.fil, err = telefil.New(); err != nil {
			return nil, err
		}
	}
	if opts.dealPricePerEpochPicker == nil {
		opts.dealPricePerEpochPicker = func(pieceSize abi.PaddedPieceSize, start, end abi.ChainEpoch) abi.TokenAmount {
			// TODO  maybe pass the draft deal proposal for a more sophisticated price picking e.g. per Provider?
			durationEpochs := big.NewInt(int64(end - start))
			pieceSizeInGiB := big.NewInt(int64(pieceSize / GiB))
			var perGiB, perGiBEpoch abi.TokenAmount
			perGiB.Mul(opts.dealPricePerGiB.Int, pieceSizeInGiB.Int)
			perGiBEpoch.Mul(opts.dealPricePerGiBEpoch.Int, new(big.Int).Mul(pieceSizeInGiB.Int, durationEpochs.Int))
			return big.Max(big.Max(perGiB, perGiBEpoch), opts.dealPricePerDeal)
		}
	}
	return &opts, nil
}

// TODO add With* option setting

package jiffy

import (
	"context"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/crypto"
)

type Wallet interface {
	Sign(context.Context, []byte) (*crypto.Signature, error)
	Address() (address.Address, error)
}

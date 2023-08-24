package car

import (
	"io"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/multiformats/go-varint"
)

var (
	_ io.WriterTo = (*Section)(nil)
	_ io.WriterTo = (*varintLength)(nil)

	// buffers pools the 8-byte long slices used to encode varint lengths.
	buffers = sync.Pool{
		New: func() any { return new([8]byte) },
	}
)

type (
	// Section represents the data in a CAR section and implements encoding of data via io.WriterTo interface
	// It automatically encodes varint-length and CID for the given dara.
	// All CIDs encoded by Section use cid.Raw codec with multihash.SHA2_256 digest.
	Section      []byte
	varintLength uint64
)

// WriteTo encodes the data bytes as varint length, plus CID of data, plus data.
// The varint length is the sum of CID in bytes plus the data length.
// The CID is encoded as cid.Raw codec with multihash.SHA2_256 digest.
func (s Section) WriteTo(out io.Writer) (int64, error) {
	// TODO convert to using streaming sum since we know the final size if hash function is fixed to SHA 256
	mh, err := multihash.Sum(s, multihash.SHA2_256, -1)
	if err != nil {
		return 0, err
	}
	cb := cid.NewCidV1(cid.Raw, mh).Bytes()
	var written int64
	// Write varint length.
	{
		l, err := varintLength(len(cb) + len(s)).WriteTo(out)
		written += l
		if err != nil {
			return written, err
		}
	}
	// Write cid byte value.
	{
		l, err := out.Write(cb)
		written += int64(l)
		if err != nil {
			return written, err
		}
	}
	// Write raw data.
	{
		l, err := out.Write(s)
		written += int64(l)
		if err != nil {
			return written, err
		}
	}
	return written, nil
}

func (l varintLength) WriteTo(out io.Writer) (int64, error) {
	buf := buffers.Get().(*[8]byte)
	defer buffers.Put(buf)
	n := varint.PutUvarint(buf[:], uint64(l))
	written, err := out.Write(buf[:n])
	return int64(written), err
}

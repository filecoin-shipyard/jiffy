package car

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyCarV1HeaderLength(t *testing.T) {
	var buf bytes.Buffer
	gotWritten, err := varintLength(len(EmptyHeaderV1Bytes[1:])).WriteTo(&buf)
	gotEncodedLength := buf.Bytes()
	wantEncodedLength := EmptyHeaderV1Bytes[:1]
	require.NoError(t, err)
	require.EqualValues(t, buf.Len(), gotWritten)
	require.Equal(t, wantEncodedLength, gotEncodedLength)
}

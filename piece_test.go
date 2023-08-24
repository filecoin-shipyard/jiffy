package jiffy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmptyHeaderV1SegmentSize(t *testing.T) {
	require.NoError(t, emptyHeaderV1Segment.Info.Size.Validate())
	require.Equal(t, emptyHeaderV1Segment.Info.Size, 128)
}

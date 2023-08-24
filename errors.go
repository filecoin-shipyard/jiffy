package jiffy

import "errors"

var (
	// ErrSegmentTooLarge signals that segment data exceeds the maximum allowed.
	ErrSegmentTooLarge = errors.New("segment data exceeds the maximum allowed")

	//ErrSegmentNotFound signals that the segment corresponding to a given piece CID is not found.
	ErrSegmentNotFound = errors.New("segment not found")
)

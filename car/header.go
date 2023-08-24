package car

// EmptyHeaderV1Bytes represents the ecoded byte value of a CARv1 header with no root CIDs.
var EmptyHeaderV1Bytes = []byte{
	0x11,                         // varint length of 17
	0xa2,                         // map of 2 items
	0x65,                         // string of length 5
	0x72, 0x6f, 0x6f, 0x74, 0x73, // "roots"
	0xf6,                                     // null
	0x67,                                     // string of length 7
	0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, // "version"
	0x01, // 1
}

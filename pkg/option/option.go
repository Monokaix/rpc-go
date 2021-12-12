package option

import "rpc-go/pkg/codec"

const MagicNumber = 0x237658

// Option is used when client and server negotiate what codec to use.
type Option struct {
	// MagicNumber marks this is a rpc call
	MagicNumber int
	// CodecType specify which codec to encode and decode
	CodecType codec.Type
}

var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.GobType,
}

package codec

import "io"

// Header is client request header
type Header struct {
	// ServerMethod is the rpc method name.
	ServerMethod string
	// Seq is an id to identify a client.
	Seq   uint64
	Error string
}

// Codec define codec to encode and decode message.
type Codec interface {
	io.Closer
	ReadHeader(header *Header) error
	ReadBody(body interface{}) error
	Write(header *Header, body interface{}) error
}

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

type NewCodecFunc func(closer io.ReadWriteCloser) Codec

var NewCodecFuncMap map[Type]NewCodecFunc

func init() {
	NewCodecFuncMap = make(map[Type]NewCodecFunc)
	NewCodecFuncMap[GobType] = NewGobCodec
}

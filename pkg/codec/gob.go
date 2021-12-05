package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// GobCodec is an implement of Codec using encoding/gob
type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	enc  *gob.Encoder
	dec  *gob.Decoder
}

var _ Codec = (*GobCodec)(nil)

func (g *GobCodec) Close() error {
	return g.conn.Close()
}

func (g *GobCodec) ReadHeader(header *Header) error {
	return g.dec.Decode(header)
}

func (g GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

func (g *GobCodec) Write(header *Header, body interface{}) (err error) {
	defer func() {
		_ = g.buf.Flush()
		if err != nil {
			_ = g.Close()
		}
	}()

	if err := g.enc.Encode(header); err != nil {
		log.Println("rpc: gob error encoding header:", err)
		return err
	}
	if err := g.enc.Encode(body); err != nil {
		log.Println("rpc: gob error encoding body:", err)
		return err
	}
	return err
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		enc:  gob.NewEncoder(buf),  // encode write data to buf for cache.
		dec:  gob.NewDecoder(conn), // decode read from conn directly.
	}
}

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"rpc-go/pkg/codec"
	"rpc-go/pkg/option"
	"sync"
)

// Client represents an RPC Client.
// There may be multiple outstanding Calls associated
// with a single Client, and a Client may be used by
// multiple goroutines simultaneously.
type Client struct {
	cc  codec.Codec
	opt *option.Option

	sending sync.Mutex // used when multi goroutines send req,to avoid unordered req
	header  codec.Header
	mu      sync.Mutex       // locked when client manage all calls
	seq     uint64           // distribute this seq to each call.Seq
	pending map[uint64]*Call // restore calls to be processed,removed when a call is replied

	closing  bool // client side set client is closed
	shutdown bool // server side set client is closed
}

var _ io.Closer = (*Client)(nil)
var ErrShutdown = errors.New("connection is shut down")

// NewClient return a new client.
func NewClient(conn net.Conn, opt *option.Option) (*Client, error) {
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}

	// send options first to determine which codec to use
	if err := json.NewEncoder(conn).Encode(&opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}

	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *option.Option) *Client {
	client := &Client{
		seq:     1, // seq starts with 1, 0 means invalid call
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}

	go client.receive()
	return client
}

func (c *Client) registryCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}

	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

func (c *Client) terminalCalls(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()

	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

// receive blocks until sever return reply.
func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil {
			break
		}

		call := c.removeCall(h.Seq)
		switch {
		// it means when client send req err occurs,and call has already been removed
		case call == nil:
			err = c.cc.ReadBody(nil)
		case h.Error != "":
			// record err msg
			call.Error = fmt.Errorf(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			call.done()
		}
	}

	// break when err occurs,and terminate pending calls
	c.terminalCalls(err)
}

// Dial connects to an RPC server at the specified network address
func Dial(network, address string, opts ...*option.Option) (client *Client, err error) {
	opt, err := parseOption(opts...)
	if err != nil {
		return nil, err
	}

	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}

	// close the connection if client is nil
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()

	return NewClient(conn, opt)
}

// Go invokes the function asynchronously.
// It returns the Call structure representing the invocation.
func (c *Client) Go(serverMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServerMethod: serverMethod,
		Args:         args,
		Reply:        reply,
		Done:         done,
	}
	c.send(call)
	return call
}

func (c *Client) send(call *Call) {
	// make sure that the client will send a complete request
	c.sending.Lock()
	defer c.sending.Unlock()

	// registry this call
	seq, err := c.registryCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}

	// prepare request header
	c.header.ServerMethod = call.ServerMethod
	c.header.Seq = seq

	// encode and send req
	if err = c.cc.Write(&c.header, call.Args); err != nil {
		call = c.removeCall(seq)
		// call may be nil, it usually means that Write partially failed,
		// client has received the response and handled
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func parseOption(opts ...*option.Option) (*option.Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return option.DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	opt := opts[0]
	opt.MagicNumber = option.DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = option.DefaultOption.CodecType
	}
	return opt, nil
}

func (c *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-c.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return !c.closing && !c.shutdown
}

package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"sync"

	"rpc-go/pkg/codec"
)

const MagicNumber = 0x237658

// Server is rpc server
type Server struct{}

func NewServer() *Server {
	return &Server{}
}

// DefaultServer is a default server for user to use.
var DefaultServer = NewServer()

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

// invalidRequest is sent when error occurs.
var invalidRequest = struct{}{}

// Accept accepts new connection from client forever
// and process each conn in a new goroutine.
func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
		}
		s.ServeConn(conn)
	}
}

func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// ServeConn process each connection
// it blocks until client comes new request.
func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()

	var opt Option
	// use json here because we need to get concrete codec first
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	s.serveCodec(f(conn))
}

func (s *Server) serveCodec(cc codec.Codec) {
	// each goroutine process each request so mutex is needed for concurrent request process
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)

	for {
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			req.header.Error = err.Error()
			s.sendResponse(cc, req.header, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go s.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	header *codec.Header
	// TODO: we haven't define req and resp format currently,so interface is used.
	body, replyBody reflect.Value
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{header: h}
	req.body = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.body.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
		return nil, err
	}
	return req, nil
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) handleRequest(cc codec.Codec, req *request, mutex *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println("***",req.header, req.body.Elem())
	req.replyBody = reflect.ValueOf(fmt.Sprintf("received rpc call %d", req.header.Seq))
	s.sendResponse(cc, req.header, req.replyBody.Interface(), mutex)
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()

	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

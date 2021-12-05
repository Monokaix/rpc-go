package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"rpc-go/pkg/codec"
	"rpc-go/pkg/server"
	"time"
)

func main() {
	addr := make(chan string)
	go startServer(addr)

	conn, _ := net.Dial("tcp", <-addr)
	defer func() {
		_ = conn.Close()
	}()

	time.Sleep(time.Second)
	// send option
	_ = json.NewEncoder(conn).Encode(server.DefaultOption)
	cc := codec.NewGobCodec(conn)
	for i := 0; i < 5; i++ {
		h := &codec.Header{Seq: uint64(i), ServerMethod: "foo-bar"}
		_ = cc.Write(h, fmt.Sprintf("rpc req code %d", h.Seq))
		_ = cc.ReadHeader(h)
		// header should be not modified because server doesn't change anything.
		//fmt.Println("",h)
		var reply string
		_ = cc.ReadBody(&reply)
		fmt.Println("reply:", reply)
	}

}

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalln("listen err", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.Accept(l)
}

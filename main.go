package main

import (
	"fmt"
	"log"
	"net"
	"rpc-go/pkg/client"
	"rpc-go/pkg/server"
	"sync"
	"time"
)

func main() {
	addr := make(chan string)
	go startServer(addr)

	client, err := client.Dial("tcp", <-addr)
	if err != nil {
		log.Fatalln("dial err:", err)
	}
	defer func() {
		_ = client.Close()
	}()

	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("rpc req %d", i)
			var reply string
			if err := client.Call("foo.bar", args, &reply); err != nil {
				log.Fatalln("call foo.bar error:", err)
			}
			log.Println("reply:", reply)
		}(i)
	}
	wg.Wait()
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

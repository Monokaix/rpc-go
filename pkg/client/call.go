package client

// Call represents an active RPC.
// 或者理解为一次RPC请求
type Call struct {
	Seq          uint64      // request id
	ServerMethod string      // rpc method
	Args         interface{} // args to a rpc method
	Reply        interface{} // reply from server
	Error        error
	Done         chan *Call // notify when a rpc call is done(get reply from server)
}

// done is called when this rpc call is done.
func (c *Call) done() {
	c.Done <- c
}

package server

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

// methodType represents a method.
type methodType struct {
	method    reflect.Method // a concrete method
	ArgType   reflect.Type   // in args type. type here,not real value,real value is defined in req.
	ReplyType reflect.Type   // out args type
	numCalls  uint64         // for statistics
}

// NumCalls return how many times this method is called.
func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) newReplyv() reflect.Value {
	// reply must be a pointer type
	replv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replv
}

// service represents a service,also a struct.
type service struct {
	name     string        // the name of this service/struct
	typ      reflect.Type  // the struct's type
	receiver reflect.Value // receiver, the instance of this service/struct
	method   map[string]*methodType
}

func newService(receiver interface{}) *service {
	s := &service{
		typ:      reflect.TypeOf(receiver),
		receiver: reflect.ValueOf(receiver),
		method:   nil,
	}
	s.name = reflect.Indirect(s.receiver).Type().Name()
	if !ast.IsExported(s.name) {

		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	s.registerMethods()
	return s
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mtype := method.Type
		// in args=3,out args=1
		if mtype.NumIn() != 3 || mtype.NumOut() != 1 {
			continue
		}
		// return val should be error type
		if mtype.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mtype.In(1), mtype.In(2)
		// Type should be exported, Upper case struct or built-in type.
		if !isExportedOrBuildType(argType) || !isExportedOrBuildType(replyType) {
			continue
		}
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuildType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	returnValues := f.Call([]reflect.Value{s.receiver, argv, replyv})
	// return value is error
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

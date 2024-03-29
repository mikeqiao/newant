package rpc

import (
	"errors"
	"fmt"
	"runtime"

	conf "github.com/mikeqiao/newant/config"
	"github.com/mikeqiao/newant/log"
	"github.com/mikeqiao/newant/net"
)

type Return struct {
	ret interface{}
	err error
	cb  interface{}
}

type Client struct {
	s               *Server
	ChanAsynRet     chan *Return
	pendingAsynCall int
}

func (c *Client) Init(l int32, s *Server) {
	c.s = s
	c.ChanAsynRet = make(chan *Return, l)
}

func (c *Client) CallBack(r *Return) {
	c.pendingAsynCall--
	//	log.Debug(" start handle callback time:%v", time.Now().String())
	execCb(r)
	//	log.Debug(" end handle callback time:%v", time.Now().String())
}

func (c *Client) call(ci *CallInfo) (err error) {
	select {
	case c.s.ChanCall <- ci:
	default:
		err = errors.New("chanrpc channel full")
	}
	return
}

func (c *Client) GetFunc(id uint32) (fc interface{}, err error) {

	if nil == c.s {
		err = fmt.Errorf("rpc server is nil")
		log.Error("err:%v", err)
		return
	}
	fc = c.s.functions[id]
	if fc == nil {
		err = fmt.Errorf("function id %v: function not registered", id)
		log.Error("err:%v", err)
	}
	return
}

func NewClient(l int32, s *Server) *Client {
	c := new(Client)
	c.Init(l, s)
	return c
}

//模块间异步调用
func (c *Client) CallAsyn(mid int64, fid, did uint32, cb interface{}, in interface{}, data *net.UserData) {
	// too many calls
	if c.pendingAsynCall >= cap(c.ChanAsynRet) && nil != cb {
		log.Debug("too mana calls:%v", c.pendingAsynCall, cap(c.ChanAsynRet))
		execCb(&Return{err: errors.New("too many calls"), cb: cb})
		return
	}
	f, err := c.GetFunc(did)
	if err != nil {
		log.Debug("err  do id:%v, func id:%v", did, fid)
		if nil != cb {
			c.ChanAsynRet <- &Return{err: err, cb: cb}
		}
		return
	}
	err = c.call(&CallInfo{
		Mid:     mid,
		Fid:     fid,
		Did:     did,
		f:       f,
		Args:    in,
		Data:    data,
		chanRet: c.ChanAsynRet,
		Cb:      cb,
	})
	if err != nil && nil != cb {
		log.Debug("err call")
		c.ChanAsynRet <- &Return{err: err, cb: cb}
		return
	}
	if nil != cb {
		c.pendingAsynCall++
	}

}

func assert(i interface{}) []interface{} {
	if i == nil {
		return nil
	} else {
		return i.([]interface{})
	}
}

func execCb(ri *Return) {
	defer func() {
		if r := recover(); r != nil {
			if conf.Config.LenStackBuf > 0 {
				buf := make([]byte, conf.Config.LenStackBuf)
				l := runtime.Stack(buf, false)
				log.Error("%v: %s", r, buf[:l])
			} else {
				log.Error("%v", r)
			}
		}
	}()
	if nil == ri.cb {
		return
	}
	f, ok := ri.cb.(func(interface{}, error))
	if ok {
		f(ri.ret, ri.err)
	} else {
		log.Error("err cb format")
	}
	return
}

//不同进程间调用
func (c *Client) CallRemote(cb interface{}) {

}

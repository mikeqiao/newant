package pb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/mikeqiao/newant/db/mysql"
	"github.com/mikeqiao/newant/log"
	mod "github.com/mikeqiao/newant/module"
	"github.com/mikeqiao/newant/net"
	"github.com/mikeqiao/newant/net/msgtype"
	msg "github.com/mikeqiao/newant/net/proto"
	"github.com/mikeqiao/newant/network"
)

// -------------------------
// | id | protobuf message |
// -------------------------
type Processor struct {
	littleEndian bool
	msgInfo      map[uint32]*MsgInfo
	msgID        map[reflect.Type]uint32
}

type MsgInfo struct {
	msgType    reflect.Type
	msgHandler MsgHandler
	fid        uint32
}

type MsgHandler func(msg interface{}, data *net.UserData)

func NewProcessor() *Processor {
	p := new(Processor)
	p.littleEndian = false
	p.msgInfo = make(map[uint32]*MsgInfo)
	p.msgID = make(map[reflect.Type]uint32)
	p.baseMsg()
	return p
}

func (p *Processor) baseMsg() {

	p.Register(&msg.NewConnect{}, msgtype.NewConnect)
	p.Register(&msg.DelConnect{}, msgtype.DelConnect)
	p.Register(&msg.ServerDelConnect{}, msgtype.ServerDelConnect)
	p.Register(&msg.ServerTick{}, msgtype.ServerTick)
	p.Register(&msg.ServerLoginRQ{}, msgtype.ServerLoginRQ)
	p.Register(&msg.ServerLoginRS{}, msgtype.ServerLoginRS)
	p.Register(&msg.ServerRegister{}, msgtype.ServerRegister)
	p.Register(&msg.ServerDelFunc{}, msgtype.ServerDelFunc)
	p.Register(&msg.ServerCall{}, msgtype.ServerCall)
	p.Register(&msg.ServerCallBack{}, msgtype.ServerCallBack)
	p.Register(&msg.DBServerRQ{}, msgtype.DBServerRQ)
	p.Register(&msg.DBServerRS{}, msgtype.DBServerRS)

	p.SetHandler(msgtype.NewConnect, network.HandleNewConnect)
	//	p.SetHandler(msgtype.DelConnect, network.HandleDelConnect)
	p.SetHandler(msgtype.ServerDelConnect, network.HandleServerDelConnect)
	p.SetHandler(msgtype.ServerTick, network.HandleServerTick)
	p.SetHandler(msgtype.ServerLoginRQ, network.HandleServerLoginRQ)
	p.SetHandler(msgtype.ServerLoginRS, network.HandleServerLoginRS)
	p.SetHandler(msgtype.ServerRegister, network.HandleServerRegister)
	p.SetHandler(msgtype.ServerDelFunc, network.HandleServerDelFunc)
	p.SetHandler(msgtype.ServerCall, network.HandleServerCall)
	p.SetHandler(msgtype.ServerCallBack, network.HandleServerCallBack)

}

func (p *Processor) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

func (p *Processor) Register(msg interface{}, id uint32) uint32 {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("protobuf message pointer required")
	}
	if _, ok := p.msgInfo[id]; ok {
		log.Fatal("message id%v type %s is already registered", id, msgType)
	}
	if _, ok := p.msgID[msgType]; ok {
		log.Fatal("message %s is already registered", msgType)
	}
	if id >= math.MaxUint16 {
		log.Fatal("too many protobuf messages (max = %v)", math.MaxUint16)
	}
	i := new(MsgInfo)
	i.msgType = msgType
	p.msgInfo[id] = i
	p.msgID[msgType] = id
	if nil != mysql.DB {
		mysql.DB.Register(msg)
	}
	return id
}

func (p *Processor) SetRouter(id uint32, fid uint32) {
	if v, ok := p.msgInfo[id]; !ok || nil == v {
		log.Fatal("message %s not registered", id)
	} else {
		v.fid = fid
	}
}

func (p *Processor) SetHandler(id uint32, msgHandler MsgHandler) {
	if v, ok := p.msgInfo[id]; !ok || nil == v {
		log.Fatal("message %s not registered", id)
	} else {
		v.msgHandler = msgHandler
	}
}

func (p *Processor) Route(id uint32, msg interface{}, data *net.UserData) error {

	i, ok := p.msgInfo[id]
	if !ok || nil == i {
		return fmt.Errorf("message id:%v %s not registered", id)
	}
	if i.msgHandler != nil {
		i.msgHandler(msg, data)
	} else {
		if 0 != i.fid {
			cb := func(in interface{}, e error) {
				if nil != in && nil != data && nil != data.Agent {
					data.Agent.WriteMsg(in)
				}
			}
			log.Debug("fid:%v, did:%v, msg:%v, data:%v", i.fid, i.fid, msg, data)
			mod.RPC.Route(i.fid, i.fid, cb, msg, data)
		} else {
			return fmt.Errorf(" msgid:%v, handler is nil :%v", id, i)
		}
	}
	return nil
}

func (p *Processor) Unmarshal(data []byte) (uint32, interface{}, error) {
	if len(data) < 4 {
		return 0, nil, errors.New("protobuf data too short")
	}
	// id
	var id uint32
	if p.littleEndian {
		id = binary.LittleEndian.Uint32(data)
	} else {
		id = binary.BigEndian.Uint32(data)
	}

	i, ok := p.msgInfo[id]
	if !ok {
		return id, nil, fmt.Errorf("message id %v not registered", id)
	}
	// msg
	msg := reflect.New(i.msgType.Elem()).Interface()
	return id, msg, proto.UnmarshalMerge(data[4:], msg.(proto.Message))
}

// goroutine safe
func (p *Processor) Marshal(msg interface{}) (uint32, [][]byte, error) {
	msgType := reflect.TypeOf(msg)
	// id
	_id, ok := p.msgID[msgType]
	if !ok {
		err := fmt.Errorf("message %s not registered", msgType)
		return _id, nil, err
	}

	id := make([]byte, 4)
	if p.littleEndian {
		binary.LittleEndian.PutUint32(id, _id)
	} else {
		binary.BigEndian.PutUint32(id, _id)
	}
	// data
	data, err := proto.Marshal(msg.(proto.Message))
	return _id, [][]byte{id, data}, err
}

func (p *Processor) Range(f func(id uint16, t reflect.Type)) {
	for id, i := range p.msgInfo {
		f(uint16(id), i.msgType)
	}
}

func (p *Processor) GetMsg(id uint32) interface{} {
	i, ok := p.msgInfo[id]
	if !ok {
		log.Error("message id %v not registered", id)
		return nil
	}
	msg := reflect.New(i.msgType.Elem()).Interface()
	return msg
}

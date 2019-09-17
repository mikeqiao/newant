package net

type Processor interface {
	//发送处理
	Route(id uint32, msg interface{}, userData *UserData) error
	//解包数据
	Unmarshal(data []byte) (uint32, interface{}, error)
	//打包数据
	Marshal(msg interface{}) (uint32, [][]byte, error)

	Register(msg interface{}, id uint32) uint32

	GetMsg(id uint32) interface{}
}

package msgtype

const (
	_None uint32 = iota
	NewConnect
	DelConnect
	ServerDelConnect
	ServerTick
	ServerLoginRQ
	ServerLoginRS
	ServerRegister
	ServerDelFunc
	ServerCall
	ServerCallBack
	DBServerRQ
	DBServerRS
)

package rpc

import ()

//事件处理
type Handler interface {
	//连接建立
	ConnectioHandle(e *Event, err error)

	// 异常处理
	ExceptionHandle(e *Event, err error)
	// 处理时间
	HandleEvent(e *Event, err error)
	/* 连接断开*/
	ConnectionRemove(e *Event, err error)
	//心跳事件
	HeartEvent(e *Event, err error)
}

//包,处理包数据编码，解码
type Pack interface {
	Encode() []byte                                             //将包数据编码为byte
	Decode(buffer *[]byte, size uint32) ([]Pack, uint32, error) //批量解包
}

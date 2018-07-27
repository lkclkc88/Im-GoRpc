package rpc

import (
	Loger "github.com/lkclkc88/log4g"
	"sync/atomic"
	"time"
)

var log = Loger.GetLogger()

type EVENTSTATUS uint8

const (
	_                EVENTSTATUS = iota
	EVENT_ESTABLISH              //建立连接事件
	EVENT_HEART                  //心跳事件
	EVENT_READ                   //读事件
	EVENT_WRITE                  //写事件
	EVENT_DISCONNECT             //断开连接事件
	EVENT_EXCEPTION              //异常事件
)

var seq_index uint32 = 0
var seq_tmp uint32 = (1 << 27) - 1

//var seq_tmp1 uint32 = 0

//构建seq，seq用于请求唯一标示，并且在同一个socket中使用，因此只需要保证几秒内数据不重复即可,因此只需要保证1分钟内数据不重复
func BuildSeq() uint32 {
	index := atomic.AddUint32(&seq_index, 1)
	now := time.Now()
	result := uint32((now.Second() << 26)) + (index & seq_tmp)
	return uint32(result)
}

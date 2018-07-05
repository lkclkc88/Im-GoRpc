package rpc

import (
	Loger "github.com/lkclkc88/log4g"
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

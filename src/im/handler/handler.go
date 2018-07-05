package handler

//import (
//	Loger "github.com/lkclkc88/log4g"
//	con "im/go/rpc"
//	p "im/go/rpc/imPack"
//	"runtime/debug"
//)
//
//var log = Loger.GetLogger()
//
//type DefaultHandle struct {
//}
//
////链接建立
//func (d DefaultHandle) ConnectioHandle(e *con.Event, err error) {
//
//	log.Info(" connection 建立 ", (*e.Conn.Conn).RemoteAddr(), (*e.Conn.Conn).LocalAddr())
//
//}
//
////链接执行异常
//func (d DefaultHandle) ExceptionHandle(e *con.Event, err error) {
//	log.Error(err)
//	e.Conn.Close()
//}
//
////处理事件
//func (d DefaultHandle) HandleEvent(e *con.Event, err error) {
//	defer func() {
//		if err := recover(); err != nil {
//			log.Error(err)
//			log.Error(string(debug.Stack()))
//		}
//	}()
//	if e.EType == con.EVENT_READ {
//		switch (*e.Pack).(type) {
//		case p.ImPack:
//			pack := (*e.Pack).(p.ImPack)
//			header := pack.Header
//			if header.Cmd == 0 {
//				log.Debug(" reciver heart message", (*e.Conn.Conn).RemoteAddr().String(), header.Cmd, header.Seq, header.Len)
//				//心跳
//				e.Conn.Send(e.Pack)
//			} else {
//				log.Debug(" reciver data message", (*e.Conn.Conn).RemoteAddr().String(), header.Cmd, header.Seq, header.Len)
//
//				e.Conn.Send(e.Pack)
//			}
//		}
//	}
//
//}
//
//func (d DefaultHandle) HeartEvent(e *con.Event, err error) {
//	if e.EType == con.EVENT_HEART {
//		e.Conn.Close()
//		log.Error("---------hearch check timeout close connection")
//	}
//
//}
//
//func (d DefaultHandle) ConnectionRemove(e *con.Event, err error) {
//	log.Info(" connection 断开 ", (*e.Conn.Conn).RemoteAddr(), (*e.Conn.Conn).LocalAddr())
//}

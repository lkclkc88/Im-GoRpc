package rpc

import (
	"errors"
	"net"
)

//服务器
type Server struct {
	Addr         string
	HeartTime    int //心跳时间，单位秒，默认5秒
	Handler      *Handler
	Pack         *Pack
	maxWorker    uint32 //最大执行worker,默认150个
	maxQueueSize uint32 //最大等待队列数，默认1000

	workerPool *WorkerPool
}

func NewServer() *Server {
	return &Server{maxWorker: 150, HeartTime: 5, maxQueueSize: 1000}
}

/*开始服务端 */
func (server *Server) InitServer() error {

	if "" == server.Addr {
		return errors.New("Addr can't be nil")
	}
	if nil == server.Handler {
		return errors.New(" handler can't be nil")
	}
	if nil == server.Pack {
		return errors.New("pack can't be nil")
	}
	log.Info(" start   server ", server.Addr)
	ln, err := net.Listen("tcp", server.Addr)
	if err == nil {
		log.Info(" start server success")
		server.workerPool = NewWorkerPool(server.maxQueueSize, server.maxWorker)
		go server.listen(ln)
	} else {
		log.Info(" start server failed")
	}
	return err
}

/* 监听链接 */
func (server *Server) listen(listen net.Listener) {
	for {
		conn, err := listen.Accept()
		if err == nil {
			log.Debug(" 建立链接  ", conn.RemoteAddr().String())

			c := NewConnection(&conn, server.HeartTime, server.Handler, server.Pack, server.workerPool)
			c.startHeartCheck()
			go readHandle(&c)
			//连接事件
			c.submitEventPool(nil, EVENT_ESTABLISH, (*server.Handler).ConnectioHandle, nil)
		}

	}
}

/*处理读数据 */
func readHandle(c *Connection) {
	for {
		ps, err := readPack(c)
		if err != nil {
			log.Error(err)
			c.submitEventPool(nil, EVENT_EXCEPTION, (*c.handler).ExceptionHandle, err)
			c.Close()
			return
		}
		if nil == ps {
			continue
		}
		i := 0
		l := len(*ps)
		for ; i < l; i++ {
			//读事件
			c.submitEventPool(&(*ps)[i], EVENT_READ, (*c.handler).HandleEvent, nil)
		}
	}
}

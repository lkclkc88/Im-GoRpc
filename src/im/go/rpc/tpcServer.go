package rpc

import (
	"errors"
	"net"
	"time"
)

//服务器
type Server struct {
	Addr         string
	HeartTime    int //心跳时间，单位秒，默认5秒
	Handler      *Handler
	Pack         *Pack
	MaxWorker    uint32 //最大执行worker,默认150个
	MaxQueueSize uint32 //最大等待队列数，默认1000
	workerPool   *WorkerPool
	//	AsyncReadWrite bool //异步读写
}

func NewServer() *Server {
	return &Server{MaxWorker: 150, HeartTime: 5, MaxQueueSize: 1000}
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
		server.workerPool = NewWorkerPool(server.MaxQueueSize, server.MaxWorker)
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
			tcpConn, ok := conn.(*net.TCPConn)
			if ok {
				tcpConn.SetNoDelay(false)
				tcpConn.SetWriteBuffer(1024 * 16)

				if server.HeartTime > 0 {
					tcpConn.SetKeepAlive(true)
					tcpConn.SetKeepAlivePeriod(time.Duration(server.HeartTime+1) * time.Second)
				} else {
					tcpConn.SetKeepAlive(false)
				}
			}

			c := NewConnection(&conn, server.HeartTime, server.Handler, server.Pack, server.workerPool)
			c.startHeartCheck()
			log.Info(" read Handler")
			go c.readHandle()
			//连接事件
			c.submitEventPool(nil, EVENT_ESTABLISH, (*server.Handler).ConnectioHandle, nil)
		}

	}
}

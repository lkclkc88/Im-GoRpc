package rpc

import (
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

//客户端配置信息
type ClientConfig struct {
	Host      string //主机名
	Port      int    //端口
	HeartTime int    //心跳时间
	Weight    uint8  //权重。默认1
}

//tcp客户端池
type ClientPool struct {
	lock         sync.RWMutex
	Configs      []*ClientConfig    //配置
	Status       int                //状态，1 运行中，0未运行，2关闭
	Conns        []*Connection      //链接
	Handler      *Handler           //处理器
	Pack         *Pack              //协议包类，支持编码，解码包数据
	MaxWorker    uint32             //最大执行worker,默认150个
	MaxQueueSize uint32             //最大等待队列数，默认1000
	workerPool   *WorkerPool        //工作协程池
	connTimeOut  time.Duration      //连接超时
	reConnCh     chan *ClientConfig //重连客户端配置队列

	clientMap map[*ClientConfig]*Connection //客户端配置与连接映射关系
}

//构建tcp客户端
func NewClientPool() *ClientPool {
	return &ClientPool{MaxWorker: 150, MaxQueueSize: 2000, Status: 0, connTimeOut: 3 * time.Second}
}

//初始化tcp客户端，初始工作池，建立连接
func (c *ClientPool) InitClient() bool {
	if c.Configs == nil || len(c.Configs) < 1 || nil == c.Handler || nil == c.Pack || nil != c.Conns {
		return false
	}
	if c.MaxWorker < 1 {
		c.MaxWorker = 150
	}
	if c.MaxQueueSize < 1 {
		c.MaxQueueSize = 1000
	}
	c.workerPool = NewWorkerPool(c.MaxQueueSize, c.MaxWorker)
	c.batchConn()

	return true
}

//批量连接客户端，
func (c *ClientPool) batchConn() {
	if nil != c.Conns {
		conns := make([]*Connection, len(c.Configs))
		ch := make(chan *ClientConfig, len(c.Configs))
		for _, v := range c.Configs {
			conn := c.connectionClient(v)
			if nil != conn {
				c.putConn(conn, v)
			} else {
				ch <- v
			}
		}
		c.Conns = conns
		go c.reConnAll()
	}
}

//通过客户端配置信息建立连接
func (c *ClientPool) connectionClient(config *ClientConfig) *Connection {
	addr := config.Host + ":" + strconv.Itoa(config.Port)
	conn, err := net.DialTimeout("tcp", addr, c.connTimeOut)
	if nil != err {
		log.Error(err)
		return nil
	}
	result := NewConnection(&conn, config.HeartTime, c.Handler, c.Pack, c.workerPool)
	return result
}

//放入连接池
func (c *ClientPool) putConn(con *Connection, config *ClientConfig) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.Conns = append(c.Conns, con)
	c.clientMap[config] = con

}

//延迟重试
func (c *ClientPool) lazyReconn(ch chan *ClientConfig) {
	for {
		tmp := <-ch
		time.Sleep(3 * time.Second)
		c.reConnCh <- tmp
	}
}

// 重连
func (c *ClientPool) reConnAll() {
	lazyCh := make(chan *ClientConfig, len(c.Configs))
	go c.lazyReconn(lazyCh)
	for {
		tmp := <-c.reConnCh
		if nil != tmp {
			con := c.connectionClient(tmp)
			if nil != con {

				//重连成功，放入连接池
				c.putConn(con, tmp)
			} else {
				//重连失败，放入延迟重试队列
				lazyCh <- tmp
			}
		}
	}
}

func (c *ClientPool) deleteConnection(con *Connection) {
	for i, v := range c.Conns {
		if v == con {
			c.lock.Lock()
			defer c.lock.Unlock()
			size := len(c.Conns)
			if i == size-1 {
				c.Conns = c.Conns[:i]
			} else {
				c.Conns = append(c.Conns[:i], c.Conns[i+1:]...)
			}
		}
	}
}

//重连连接，应对连接断开之后，重新连接的情况。
func (c *ClientPool) reConn(conn *Connection) {
	//从列表中删除连接
	c.deleteConnection(conn)
	//通过connection查找到客户端配置信息，将配置信息发送到重连队列中
	for k, v := range c.clientMap {
		if v == conn {
			c.reConnCh <- k
		}
	}
}

func (c *ClientPool) removeConfig(config *ClientConfig) {

	for k, v := range c.clientMap {
		if strings.EqualFold(k.Host, config.Host) && k.Port == config.Port {
			delete(c.clientMap, k)
			c.deleteConnection(v)
		}
	}
}

////移除连接，如果连接没有关闭，延迟5秒关闭
//func (c *ClientPool) removeConn(con *Connection) {
//
//	if !con.IsClose() {
//		go func() {
//			time.Sleep(5 * time.Second)
//			con.Close()
//		}()
//	}
//	c.lock.Lock()
//	for
//
//}

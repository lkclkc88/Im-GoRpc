package rpc

import (
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//客户端配置信息
type ClientConfig struct {
	Host      string //主机名
	Port      int    //端口
	HeartTime int    //心跳时间
	Weight    uint8  //权重。默认1

	status uint32 //状态，0是有效状态，1是无效状态，应对删除的情况，默认0，
}

//tcp客户端池
type ClientPool struct {
	lock          sync.RWMutex
	Configs       []*ClientConfig    //配置
	Status        int                //状态，1 运行中，0未运行，2关闭
	Conns         []*Connection      //链接
	ClientHandler *ClientHandler     //处理器
	handler       *Handler           //处理器，实现断线重连逻辑
	Pack          *Pack              //协议包类，支持编码，解码包数据
	MaxWorker     uint32             //最大执行worker,默认150个
	MaxQueueSize  uint32             //最大等待队列数，默认1000
	workerPool    *WorkerPool        //工作协程池
	connTimeOut   time.Duration      //连接超时
	reConnCh      chan *ClientConfig //重连客户端配置队列

	clientMap map[*ClientConfig]*Connection //客户端配置与连接映射关系
}

//构建tcp客户端
func NewClientPool() *ClientPool {
	return &ClientPool{MaxWorker: 150, MaxQueueSize: 2000, Status: 0, connTimeOut: 3 * time.Second}
}

//初始化tcp客户端，初始工作池，建立连接
func (c *ClientPool) InitClient() bool {
	if c.Configs == nil || len(c.Configs) < 1 || nil == c.ClientHandler || nil == c.Pack || nil != c.Conns {
		log.Warn(" configs or ClientHandler,pack is nil ,or  conns is not nil")
		return false
	}
	if c.MaxWorker < 1 {
		c.MaxWorker = 150
	}
	if c.MaxQueueSize < 1 {
		c.MaxQueueSize = 1000
	}
	var handler Handler = clientDefaultHandler{clientPool: c, handler: c.ClientHandler}

	c.handler = &handler

	c.workerPool = NewWorkerPool(c.MaxQueueSize, c.MaxWorker)
	c.reConnCh = make(chan *ClientConfig, len(c.Configs))
	c.clientMap = make(map[*ClientConfig]*Connection)
	c.Conns = make([]*Connection, 0)
	c.batchConn()

	return true
}

//批量连接客户端，
func (c *ClientPool) batchConn() {
	log.Debug(" batchonn client pool", len(c.Conns))
	if nil == c.Conns || len(c.Conns) < 1 {
		for _, v := range c.Configs {
			conn := c.connectClient(v)
			if nil == conn {
				c.reConnCh <- v
			}
		}
		go c.reConnAll()
	}
}

//通过客户端配置信息建立连接,建立配置与连接对应关系
func (c *ClientPool) connectClient(config *ClientConfig) *Connection {
	addr := config.Host + ":" + strconv.Itoa(config.Port)
	log.Debug(" start connect ", addr)
	conn, err := net.DialTimeout("tcp", addr, c.connTimeOut)
	if nil != err {
		log.Error(" connect failed ", addr, err)
		return nil
	}
	result := NewConnection(&conn, config.HeartTime, c.handler, c.Pack, c.workerPool)
	log.Debug("start start")
	result.start()
	log.Debug("start end")
	c.lock.Lock()
	defer c.lock.Unlock()
	c.clientMap[config] = result
	c.Conns = append(c.Conns, result)
	return result
}

//延迟重试
func (c *ClientPool) lazyReconn(ch chan *ClientConfig) {
	for {
		tmp := <-ch
		status := atomic.LoadUint32(&(tmp.status))
		if status == 0 { //判断连接是否已经移除
			time.Sleep(3 * time.Second)
			c.reConnCh <- tmp
		}
	}
}

// 重连
func (c *ClientPool) reConnAll() {
	lazyCh := make(chan *ClientConfig, len(c.Configs))
	go c.lazyReconn(lazyCh)
	for {
		tmp := <-c.reConnCh
		if nil != tmp {
			status := atomic.LoadUint32(&(tmp.status))
			if status == 0 {
				log.Debug("  reconn  ", tmp.Host, tmp.Port, tmp)
				con := c.connectClient(tmp)
				if nil == con {
					//重连失败，放入延迟重试队列
					lazyCh <- tmp
				}
			} else {
				log.Debug(" config is delete ,don't reconn", tmp)
			}
		}
	}
}

//删除连接
func (c *ClientPool) deleteConnection(con *Connection) {
	for i, v := range c.Conns {
		if v == con {
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
func (c *ClientPool) ReConn(conn *Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()
	//从列表中删除连接
	c.deleteConnection(conn)
	//通过connection查找到客户端配置信息，将配置信息发送到重连队列中
	for k, v := range c.clientMap {
		if v == conn {
			delete(c.clientMap, k)
			c.reConnCh <- k
			break
		}
	}
}

//删除配置
func (c *ClientPool) deleteConfig(config *ClientConfig) *ClientConfig {
	i, size := 0, len(c.Configs)
	for ; i < size; i++ {
		v := c.Configs[i]
		if strings.EqualFold(v.Host, config.Host) && v.Port == config.Port {
			if i+1 < size {
				c.Configs = append(c.Configs[:i], c.Configs[i+1:]...)
			} else {
				c.Configs = c.Configs[:i]
			}
			return v
		}
	}

	return nil
}

//移除配置，应对删除节点的情况，根据host,port确定服务
func (c *ClientPool) RemoveConfig(config *ClientConfig) {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.Debug(c.clientMap, " --- ", c.Configs, " --- ", c.Conns)
	config = c.deleteConfig(config)
	if nil != config {
		log.Debug(" remove  ", config)
		atomic.CompareAndSwapUint32(&config.status, 0, 1)
		delete(c.clientMap, config)
	}
}

//添加配置
func (c *ClientPool) AddConfig(config *ClientConfig) {
	exists := false
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, v := range c.Configs {
		if strings.EqualFold(v.Host, config.Host) && v.Port == config.Port {
			exists = true
		}
	}
	if !exists {
		c.Configs = append(c.Configs, config)
		c.reConnCh <- config
	} else {
		log.Debug(" exists config ", config)
	}
}

//发送消息
func (c *ClientPool) Send(pack *Pack) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	i := 0
	size := len(c.Conns)
	for i < 5 && i < size {
		num := rand.Uint32()
		index := int(num) % size
		conn := c.Conns[index]
		if !conn.IsClose() {
			err := conn.Send(pack)
			if nil == err {
				return
			} else {
				log.Error(err)
			}
		}
		i++
	}
}

//同步发送消息
func (c *ClientPool) SyncSend(pack *Pack, timeOut time.Duration) *Pack {
	c.lock.RLock()
	defer c.lock.RUnlock()
	i := 0
	for i < 5 {
		num := rand.Uint32()
		index := int(num) % len(c.Conns)
		conn := c.Conns[index]
		if !conn.IsClose() {
			r, e := conn.SyncSend(pack, timeOut)
			if nil != e {
				log.Error(e)
				if strings.Contains(e.Error(), "timeOut") {
					return nil
				}
			} else {
				return r
			}
		}
		i++

	}
	log.Warn(" can't find client connection")
	return nil
}

//客户端handler
type ClientHandler interface {
	//构建心跳包
	BuildHeart() *Pack
	//执行事件
	HandleEvent(e *Event, err error)
	//连接建立
	ConnectioHandle(e *Event, err error)

	// 异常处理
	ExceptionHandle(e *Event, err error)
}

//客户端默认处理器，处理了断线重连的情况
type clientDefaultHandler struct {
	clientPool *ClientPool
	handler    *ClientHandler
}

//连接建立
func (h clientDefaultHandler) ConnectioHandle(e *Event, err error) {
	log.Info(" connection 建立 ", (*e.Conn.Conn).RemoteAddr(), (*e.Conn.Conn).LocalAddr())
	//客户端建立连接之后，直接发送心跳
	h.HeartEvent(e, err)
	(*h.handler).ConnectioHandle(e, err)
}

// 异常处理
func (h clientDefaultHandler) ExceptionHandle(e *Event, err error) {
	log.Error(err)
	(*h.handler).ExceptionHandle(e, err)
}

//执行事件
func (h clientDefaultHandler) HandleEvent(e *Event, err error) {
	(*h.handler).HandleEvent(e, err)
}

//心跳事件
func (h clientDefaultHandler) HeartEvent(e *Event, err error) {
	p := (*h.handler).BuildHeart()
	if nil != p {
		log.Debug(" send heart message")
		e.Conn.Send(p)
	}
}

//断开链接
func (h clientDefaultHandler) ConnectionRemove(e *Event, err error) {
	log.Info(" connection 断开 ,重连", (*e.Conn.Conn).RemoteAddr(), (*e.Conn.Conn).LocalAddr())
	h.clientPool.ReConn(e.Conn)
}

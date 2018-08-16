package rpc

import (
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	//	"container/list"
)

//客户端配置信息
type ClientConfig struct {
	Id        string //id
	Host      string //主机名
	Port      int    //端口
	HeartTime int    //心跳时间
	Weight    uint8  //权重。默认1
	status    uint32 //状态，0是有效状态，1是无效状态，应对删除的情况，默认0，
}

//获取客户端配置的唯一id
func (c *ClientConfig) GetId() string {
	if "" != c.Id {
		return c.Id
	} else {
		return c.Host + ":" + strconv.Itoa(c.Port)
	}
}

//判断客户端配置是否相等
func (c *ClientConfig) Equals(c1 *ClientConfig) bool {
	if nil != c1 {
		return c1.GetId() == c.GetId() && c1.status == c.status && c1.HeartTime == c.HeartTime && c1.Host == c.Host && c.Port == c1.Port && c1.Weight == c.Weight
	}
	return false
}

//tcp客户端池
type ClientPool struct {
	lock          sync.RWMutex
	Configs       []*ClientConfig               //配置
	Status        int                           //状态，1 运行中，0未运行，2关闭
	Conns         map[*ClientConfig]*Connection //链接
	connList      []*Connection                 //连接列表，只包含可用的连接
	ClientHandler *ClientHandler                //处理器
	handler       *Handler                      //处理器，实现断线重连逻辑
	Pack          *Pack                         //协议包类，支持编码，解码包数据
	MaxWorker     uint32                        //最大执行worker,默认150个
	MaxQueueSize  uint32                        //最大等待队列数，默认1000
	workerPool    *WorkerPool                   //工作协程池
	connTimeOut   time.Duration                 //连接超时
	reConnCh      chan *ClientConfig            //重连客户端配置队列

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
	c.Conns = make(map[*ClientConfig]*Connection)
	c.connList = make([]*Connection, 0)
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
	c.Conns[config] = result
	c.connList = append(c.connList, result)
	return result
}

// 重连
func (c *ClientPool) reConnAll() {
	for {
		tmp := <-c.reConnCh
		if nil != tmp && tmp.status == 0 {
			status := atomic.LoadUint32(&(tmp.status))
			if status == 0 {
				log.Debug("  reconn  ", tmp.Host, tmp.Port, tmp)
				con := c.connectClient(tmp)
				if nil == con {
					//重连失败，延迟3秒重新放入队列
					go func() {
						time.Sleep(3 * time.Second)
						c.reConnCh <- tmp
					}()
				}
			} else {
				log.Debug(" config is delete ,don't reconn", tmp)
			}
		}
	}
}

//删除连接,ton
func (c *ClientPool) deleteConnection(con *Connection) {
	for i, v := range c.Conns {
		if v == con {
			c.RemoveConfig(i)
		}
	}
}

//重连连接，应对连接断开之后，重新连接的情况。
func (c *ClientPool) ReConn(conn *Connection) {
	c.lock.Lock()
	defer c.lock.Unlock()
	//通过connection查找到客户端配置信息，将配置信息发送到重连队列中

	log.Debug(" config status ", c.Conns)
	for k, v := range c.Conns {
		if v == conn {
			delete(c.Conns, k)
			if k.status == 0 {
				//客户端配置状态为有效状态时，才进行重连
				c.reConnCh <- k
			} else {
				log.Debug(" config status ", k.status, " don't  reconn")
			}
			break
		}
	}
}

//删除配置
func (c *ClientPool) RemoveConfig(config *ClientConfig) *ClientConfig {
	if config == nil {
		return nil
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	i, size := 0, len(c.Configs)
	id := config.GetId()
	for ; i < size; i++ {
		v := c.Configs[i]
		if v.GetId() == id {
			//删除config
			if i+1 < size {
				c.Configs = append(c.Configs[:i], c.Configs[i+1:]...)
			} else {
				c.Configs = c.Configs[:i]
			}
			v.status = 1

			conn := c.Conns[v]
			if nil != conn {
				//删除映射关系
				delete(c.Conns, v)
				//从连接列表中删除
				length := len(c.connList)
				for index, tmp := range c.connList {
					if tmp == conn {
						if index+1 < length {
							c.connList = append(c.connList[:index], c.connList[index+1:]...)
						} else {
							c.connList = c.connList[:index]
						}
					}
				}
				go func() {
					time.Sleep(3 * time.Second)
					log.Debug(" close connection", (*conn.Conn).RemoteAddr())
					//关闭连接
					conn.Close()
				}()
			}

			return v
		}
	}

	return nil
}

func (c *ClientPool) isExists(config *ClientConfig) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()
	i, size := 0, len(c.Configs)
	id := config.GetId()
	for ; i < size; i++ {
		v := c.Configs[i]
		if v.GetId() == id {
			if v.Equals(config) {
				//数据相等，不修改数据
				return true
			}
		}
	}
	return false
}

//添加配置
func (c *ClientPool) AddConfig(config *ClientConfig) {
	if c.isExists(config) {
		log.Debug("client config is exists not add")
		return
	}
	//先从配置中删除
	c.RemoveConfig(config)
	c.lock.Lock()
	defer c.lock.Unlock()
	//然后将新的配置加入到
	c.Configs = append(c.Configs, config)
	c.reConnCh <- config
}

func (c *ClientPool) getRangeConn() *Connection {
	c.lock.RLock()
	defer c.lock.RUnlock()
	size := len(c.Conns)
	i := 0
	num := rand.Uint32()
	for i < size {
		index := int(num) % size
		conn := c.connList[index]
		if !conn.IsClose() {
			return conn
		}
		i++
		num++
	}
	return nil
}

//发送消息
func (c *ClientPool) Send(pack *Pack) {

	conn := c.getRangeConn()
	if nil != conn {
		conn.Send(pack)
	}
}

//发送到所有客户端，应对消息广播的请情况
func (c *ClientPool) SendAll(pack *Pack) {
	c.lock.RLock()
	defer c.lock.RUnlock()
	for _, v := range c.Conns {
		if !v.IsClose() {
			err := v.Send(pack)
			if nil != err {
				log.Error(err)
			}
		}
	}

}

func (c *ClientPool) SyncSendAndReturnConn(pack *Pack, timeOut time.Duration) (*Pack, *Connection) {
	conn := c.getRangeConn()
	if nil != conn {
		r, e := conn.SyncSend(pack, timeOut)
		if nil != e {
			log.Error(e)
			if strings.Contains(e.Error(), "timeOut") {
				return nil, nil
			}
		} else {
			return r, conn
		}
	}
	log.Warn(" can't find client connection")
	return nil, nil
}

//同步发送消息
func (c *ClientPool) SyncSend(pack *Pack, timeOut time.Duration) *Pack {
	result, _ := c.SyncSendAndReturnConn(pack, timeOut)
	return result
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
	//连接移除
	ConnectionRemove(e *Event, err error)
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

package rpc

import (
	"errors"
	"net"
	"sync"
	"time"
)

/*连接 ，包含连接的上线问信息 */
type Connection struct {
	Conn       *net.Conn     //网络链接
	tmpData    *swap         //临时数据，存放解包不完全的数据
	rwLock     *sync.RWMutex //锁
	heartTime  int           //单位秒，心跳事件
	lastTime   int64         //最后处理时间，单位秒
	status     bool          //状态，true激活状态，false关闭
	handler    *Handler      //事件处理
	pack       *Pack         //编码，解码接口
	workerPool *WorkerPool   //执行工作池
}

//判断连接是否关闭，如果是，返回true，否者返回false
func (c *Connection) isClose() bool {
	c.rwLock.RLock()
	defer c.rwLock.RUnlock()
	return !c.status
}

//关闭连接
func (c *Connection) Close() {
	if c.isClose() {
		return
	}
	c.rwLock.Lock()
	defer c.rwLock.Unlock()
	c.status = false
	(*c.Conn).Close()
	//断开事件
	c.submitEventPool(nil, EVENT_DISCONNECT, (*c.handler).ConnectionRemove, nil)

}

//提交到事件池
func (c *Connection) submitEventPool(p *Pack, eType EVENTSTATUS, f func(e *Event, err error), err error) {
	event := NewEvent(eType, c, p)
	eventTask := NewEventTask(event, f, err)

	var task Task = *eventTask
	c.workerPool.Submit(&task)
}

// 创建新连接
func NewConnection(conn *net.Conn, heartTime int, handler *Handler, pack *Pack, workerPool *WorkerPool) Connection {
	return Connection{
		Conn:       conn,
		rwLock:     new(sync.RWMutex),
		tmpData:    &swap{readBuff: make([]byte, 1024)},
		heartTime:  heartTime,
		lastTime:   time.Now().Unix(),
		status:     true,
		handler:    handler,
		pack:       pack,
		workerPool: workerPool,
	}
}

// 临时交换区，存放解包后遗留的socket数据
type swap struct {
	len      uint32
	readBuff []byte
}

/*更新最后请求时间 */
func (c *Connection) lockUpdateLastTime() {
	if c.heartTime > 0 {
		c.rwLock.Lock()
		defer c.rwLock.Unlock()
		c.updateLastTime()
	}
}

/*更新最后时间 */
func (c *Connection) updateLastTime() {
	if c.heartTime > 0 {
		c.lastTime = time.Now().Unix()
	}
}

/*启动心跳检查 */
func (c *Connection) startHeartCheck() {
	if c.heartTime > 0 {
		go c.heartCheck()
	}
}

// 心跳检查
func (c *Connection) heartCheck() {
	log.Debug(" start heartCheck")
	for !c.isClose() {
		t := time.Now().Unix() - c.lastTime
		it := int(t)
		if it >= c.heartTime {
			//执行心跳事件
			c.submitEventPool(nil, EVENT_HEART, (*c.handler).HeartEvent, nil)
		} else {
			//阻塞，等待心跳时间
			time.Sleep(time.Duration((c.heartTime - it)) * time.Second)
		}
	}
	log.Debug("close heartcheck")
}

/*读取协议包 */
func readPack(c *Connection) (*[]Pack, error) {
	for {
		var buff [8096]byte
		n, err := (*c.Conn).Read(buff[0:])

		if err != nil {
			// 读取数据异常，
			return nil, err
		}
		if n > 0 {
			//读取到数据,更新 心跳状态时间,解包数据
			c.lockUpdateLastTime()
			c.tmpData.readBuff = append(c.tmpData.readBuff[0:c.tmpData.len], buff[0:n]...)
			c.tmpData.len += uint32(n)
			ps, size, err := (*c.pack).Decode(&c.tmpData.readBuff, c.tmpData.len)
			if err != nil {
				//			//接包出现异常，返回异常
				return nil, err
			}
			if size > 0 {
				c.tmpData.readBuff = c.tmpData.readBuff[size:c.tmpData.len]
				c.tmpData.len -= uint32(size)

				//解包成功,返回协议包
				return &ps, nil
			}
		}
	}
}

/*
发送数据
*/
func (c *Connection) Send(p *Pack) error {
	if !c.isClose() {
		data := (*p).Encode()
		c.rwLock.Lock()

		defer c.rwLock.Unlock()
		_, err := (*c.Conn).Write(data)

		if err != nil {
			c.submitEventPool(nil, EVENT_EXCEPTION, (*c.handler).HeartEvent, err)

		} else {
			c.updateLastTime()
		}
		return err
	} else {
		return errors.New("connector is close")
	}

}

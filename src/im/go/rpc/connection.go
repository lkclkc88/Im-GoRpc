package rpc

import (
	"bufio"
	"errors"
	"net"
	"strings"
	"sync"
	atomic "sync/atomic"
	"time"
)

var swap_cache_size int = 1024 * 8

// 临时交换区，存放解包后遗留的socket数据
type swap struct {
	size     int
	readBuff []byte
	capacity int
}

func (s *swap) appendByte(buff *[]byte) {
	if nil != buff {
		b := *buff
		length := len(b)
		size := s.size + length
		if s.capacity >= size {
			copy(s.readBuff[s.size:size], b)
		} else {
			s.capacity = size + swap_cache_size
			data := make([]byte, s.capacity)
			copy(data, s.readBuff[:s.size])
			copy(data[s.size:], b)
			s.readBuff = data
		}
		s.size = size
	}
}

func (s *swap) deleteByte(n uint32) {
	s.capacity = s.capacity - int(n)
	s.size = s.size - int(n)
	s.readBuff = s.readBuff[n:]
}

/*连接 ，包含连接的上线问信息 */
type Connection struct {
	Conn       *net.Conn     //网络链接
	tmpData    *swap         //临时数据，存放解包不完全的数据
	rwLock     *sync.RWMutex //锁
	heartTime  int           //单位秒，心跳事件
	lastTime   int64         //最后处理时间，单位秒
	status     int32         //状态，1激活状态，0关闭
	handler    *Handler      //事件处理
	pack       *Pack         //编码，解码接口
	workerPool *WorkerPool   //执行工作池
	readDataCh chan *[]byte  //读数据队列
	asyncWrite bool          //异步写
	reader     *bufio.Reader
	writer     *bufio.Writer
}

// 创建新连接
func NewConnection(conn *net.Conn, heartTime int, handler *Handler, pack *Pack, workerPool *WorkerPool) *Connection {
	tmp := Connection{
		Conn:       conn,
		rwLock:     new(sync.RWMutex),
		tmpData:    &swap{readBuff: make([]byte, swap_cache_size), capacity: swap_cache_size},
		heartTime:  heartTime,
		lastTime:   time.Now().Unix(),
		status:     1,
		handler:    handler,
		pack:       pack,
		workerPool: workerPool,
		readDataCh: make(chan *[]byte, 1000),
		reader:     bufio.NewReaderSize(*conn, 4096),
		writer:     bufio.NewWriterSize(*conn, 4096),
	}
	return &tmp
}

//判断连接是否关闭，如果是，返回true，否者返回false
func (c *Connection) IsClose() bool {
	return atomic.LoadInt32(&(c.status)) == 0
}

//关闭连接
func (c *Connection) Close() {
	status := atomic.LoadInt32(&c.status)
	if status == 1 {
		if atomic.CompareAndSwapInt32(&c.status, 1, 0) {
			(*c.Conn).Close()
			//断开事件
			c.submitEventPool(nil, EVENT_DISCONNECT, (*c.handler).ConnectionRemove, nil)
		}
	}

}

//提交到事件池
func (c *Connection) submitEventPool(p *Pack, eType EVENTSTATUS, f func(e *Event, err error), err error) {
	event := NewEvent(eType, c, p)
	eventTask := NewEventTask(event, f, err)
	var task Task = *eventTask
	c.workerPool.Submit(&task)
}

/*更新最后时间 */
func (c *Connection) updateLastTime() {
	if c.heartTime > 0 {

		currentTime := time.Now().Unix()
		for {
			lastTime := atomic.LoadInt64(&c.lastTime)
			if currentTime <= lastTime {
				break
			}
			if atomic.CompareAndSwapInt64(&c.lastTime, lastTime, currentTime) {
				break
			}
		}

		var conn net.Conn = *(c.Conn)
		conn.SetDeadline(time.Now().Add(time.Duration(c.heartTime) * time.Second))
	}
}

/*启动心跳检查,通过设置读取，写入超时来决定 */
func (c *Connection) startHeartCheck() {
	if c.heartTime > 0 {
		var conn net.Conn = *(c.Conn)
		conn.SetDeadline(time.Now().Add(time.Duration(c.heartTime) * time.Second))
	}
}

// 心跳检查
func (c *Connection) heartCheck() {
	log.Debug(" start heartCheck")
	for !c.IsClose() {
		lastTime := atomic.LoadInt64(&c.lastTime)
		t := time.Now().Unix() - lastTime
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

//读取数据
func (c *Connection) read() {
	for {
		buff := make([]byte, 1024*4)
		n, err := c.reader.Read(buff)
		if err != nil {
			str := err.Error()
			if strings.Contains(str, "timeout") {
				//读超时，出发心跳时间
				c.submitEventPool(nil, EVENT_HEART, (*c.handler).HeartEvent, nil)
				//重置心跳时间
				c.updateLastTime()
				continue
			}
			// 读取数据异常，触发异常时间
			log.Error(str)
			c.submitEventPool(nil, EVENT_HEART, (*c.handler).ExceptionHandle, nil)
			c.Close()
			c.readDataCh <- nil
			return
		}
		if n > 0 {
			//读取到数据,更新 心跳状态时间,解包数据
			c.updateLastTime()
			tmp := buff[:n]
			c.readDataCh <- &tmp
		}
	}
}

func (c *Connection) readPack() (*[]Pack, error) {
	for !c.IsClose() {
		tmp := <-c.readDataCh
		if nil != tmp {
			c.tmpData.appendByte(tmp)
			buff := c.tmpData.readBuff[:c.tmpData.size]
			ps, size, err := (*c.pack).Decode(&buff, uint32(c.tmpData.size))
			if err != nil {
				log.Error(err)
				//			//接包出现异常，返回异常
				return nil, err
			}
			if size > 0 {
				c.tmpData.deleteByte(size)
				//解包成功,返回协议包
				return &ps, nil
			}
		}
	}
	return nil, nil
}

/*处理读数据 */
func (c *Connection) readHandle() {
	log.Info(" read Handler")
	go c.read()
	for !c.IsClose() {
		ps, err := c.readPack()
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

func Open(addr string) (*bufio.ReadWriter, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, nil
	}
	// 将net.Conn对象包装到bufio.ReadWriter中
	return bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn)), nil
}

/*
发送数据
*/
func (c *Connection) Send(p *Pack) error {
	if !c.IsClose() {
		data := (*p).Encode()
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

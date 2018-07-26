package rpc

import (
	"sync"
	"sync/atomic"
	"time"
)

//请求同步器

type ReqSync struct {
	store *sync.Map
	pool  *Pool
}

func NewReqSync() *ReqSync {
	//	method := func() *interface{} {
	//		var obj interface{} = make(chan *interface{}, 1)
	//		return &obj
	//	}
	return &ReqSync{store: new(sync.Map), pool: newPool(uint32(1000), newReqQueue)}
}

func (r *ReqSync) SetSync(reqId interface{}) {
	value := r.pool.get()
	req := (*value).(reqQueue)
	(&req).start()
	r.store.Store(reqId, &req)
}

func (r *ReqSync) Reset(reqId interface{}) {
	value, ok := r.store.Load(reqId)
	if ok {
		tmp := (value).(*reqQueue)
		tmp.reset()
		var obj interface{} = *tmp
		r.pool.callBack(&obj)
	}
}

//请求同步,设定超时时间
func (r *ReqSync) SyncReq(reqId interface{}, timeOut time.Duration) *interface{} {
	value, ok := r.store.Load(reqId)
	if ok {
		defer r.store.Delete(reqId)
		tmp := (value).(*reqQueue)
		return tmp.read(timeOut)
	}
	return nil

}
func (r *ReqSync) ResponseSyncReq(reqId interface{}, obj *interface{}) bool {
	value, ok := r.store.Load(reqId)
	if ok {
		tmp := (value).(*reqQueue)
		return tmp.write(obj)
	}
	return false
}

type reqQueue struct {
	queue  chan *interface{}
	status uint32 //状态，1可写，2 可读,0 初始状态
}

func newReqQueue() *interface{} {
	var obj interface{} = reqQueue{queue: make(chan *interface{}, 1), status: 0}
	return &obj
}

//设置启用
func (r *reqQueue) start() {
	atomic.StoreUint32(&r.status, 1)
}

//重置
func (r *reqQueue) reset() {
	atomic.StoreUint32(&r.status, 0)
}

//写数据
func (r *reqQueue) write(obj *interface{}) bool {
	if atomic.CompareAndSwapUint32(&r.status, 1, 2) {
		r.queue <- obj
		return true
	}
	return false
}

func (r *reqQueue) read(timeOut time.Duration) *interface{} {
	if atomic.LoadUint32(&r.status) == 0 {
		//未启用，返回空
		return nil
	}
	to := time.NewTimer(timeOut)
	defer to.Stop()

	select {
	case tmp := <-r.queue:
		//读取成功，设置未初始状态
		atomic.StoreUint32(&r.status, 0)
		return tmp
	case <-to.C:
	}
	if !atomic.CompareAndSwapUint32(&r.status, 1, 0) {
		//更改状态失败，此时状态已经可读了，读取数据
		return r.read(timeOut)
	}

	return nil
}

//对象池
type Pool struct {
	max    uint32              //最大值
	size   uint32              //当前值
	ch     chan *interface{}   //队列使用
	method func() *interface{} //产生channel的方法
}

func newPool(max uint32, method func() *interface{}) *Pool {
	if nil == method {
		return nil
	}
	if max < 1 {
		max = 100
	}
	return &Pool{max: max, method: method}
}

func (c *Pool) get() *interface{} {
	for {
		v := atomic.LoadUint32(&c.size)
		if v > 0 {
			if atomic.CompareAndSwapUint32(&c.size, v, v-1) {
				return <-c.ch
			}
		} else {
			return c.method()
		}
	}

}

//放入池中
func (c *Pool) callBack(obj *interface{}) {
	for {
		v := atomic.LoadUint32(&c.size)
		if v < c.max && atomic.CompareAndSwapUint32(&c.size, v, v+1) {
			c.ch <- obj
		}
	}
}

package rpc

import (
	"runtime/debug"
	atomic "sync/atomic"
)

func recoverErr() {
	if err := recover(); err != nil {
		log.Error(err)
		log.Error(string(debug.Stack()))
	}
}

type Task interface {
	Run()
}

//worker
type Worker struct {
	pool *WorkerPool
}

//停止
func NewWorker(pool *WorkerPool) *Worker {
	return &Worker{pool: pool}
}

//运行任务
func (w *Worker) runWorker(task *Task) {
	defer recoverErr()
	(*task).Run()
	w.pool.afterExecuteTask(task)

}

// 启动工作线程
func (w *Worker) startWorker(task *Task) {
	f := func() {
		if nil != task {
			w.runWorker(task)
		}

		for {
			task = <-w.pool.taskCh
			if nil == task {
				return
			}
			w.runWorker(task)

		}
	}
	go f()
}

// worker池
type WorkerPool struct {
	taskNum      uint32     //当前执行任务数据
	workerNum    uint32     //当前工作协程数
	queueSize    uint32     //最大队列数
	maxWorkerNum uint32     //最大执行协程数
	taskCh       chan *Task //事件channel，做队列事件
}

func NewWorkerPool(queueSize uint32, maxWorkerNum uint32) *WorkerPool {
	return &WorkerPool{queueSize: queueSize, maxWorkerNum: maxWorkerNum, taskCh: make(chan *Task, queueSize)}
}

//提交任务，如果线程池还存在空间，返回true,否者返回false
func (pool *WorkerPool) Submit(e *Task) bool {
	//	pool.increaseTaskNum()
	pool.changeTaskNum(true)
	if !pool.increaseWorker(e) {
		pool.taskCh <- e
	}
	return true
	//	}
	//	return false
}

func (pool *WorkerPool) GetTaskNum() uint32 {
	return atomic.LoadUint32(&pool.taskNum)
}

func (pool *WorkerPool) GetWorkerNum() uint32 {
	return atomic.LoadUint32(&pool.workerNum)
}

//修改任务数
func (pool *WorkerPool) changeTaskNum(increase bool) {
	for {
		taskNum := atomic.LoadUint32(&pool.taskNum)
		newNum := taskNum
		if increase {
			newNum += 1
		} else {
			newNum -= 1
		}
		if atomic.CompareAndSwapUint32(&pool.taskNum, taskNum, newNum) {
			return
		}

	}
}

////增长任务数，当前任务数小于队列数+最大协成数时，返回true,否者返回false
//func (pool *WorkerPool) increaseTaskNum() bool {
//	for {
//		taskNum := atomic.LoadUint32(&pool.taskNum)
//		if taskNum < (pool.queueSize + pool.maxWorkerNum) {
//			if atomic.CompareAndSwapUint32(&pool.taskNum, taskNum, taskNum+1) {
//				return true
//			}
//		} else {
//			return false
//		}
//	}
//}
//
//func (pool *WorkerPool) decreaseTaskNum() bool {
//	for {
//		taskNum := atomic.LoadUint32(&pool.taskNum)
//		if taskNum < (pool.queueSize + pool.maxWorkerNum) {
//			if atomic.CompareAndSwapUint32(&pool.taskNum, taskNum, taskNum-1) {
//				return true
//			}
//		} else {
//			return false
//		}
//	}
//}

//增长工作协成
func (pool *WorkerPool) increaseWorker(task *Task) bool {
	for {
		workerNum := atomic.LoadUint32(&pool.workerNum)
		taskNum := atomic.LoadUint32(&pool.taskNum)
		if workerNum < taskNum && workerNum < pool.maxWorkerNum {
			if atomic.CompareAndSwapUint32(&pool.workerNum, workerNum, workerNum+1) {
				worker := NewWorker(pool)
				worker.startWorker(task)
				return true
			}
		} else {
			return false
		}
	}

}

func (pool *WorkerPool) decreaseWorker() bool {
	for {
		workerNum := atomic.LoadUint32(&pool.workerNum)
		taskNum := atomic.LoadUint32(&pool.taskNum)
		if workerNum > taskNum && workerNum > 0 {
			if atomic.CompareAndSwapUint32(&pool.workerNum, workerNum, workerNum-1) {
				pool.taskCh <- nil
				return true
			}
		} else {
			return false
		}
	}

}

func (pool *WorkerPool) afterExecuteTask(task *Task) {
	//	pool.decreaseTaskNum()
	pool.changeTaskNum(false)
	pool.decreaseWorker()
}

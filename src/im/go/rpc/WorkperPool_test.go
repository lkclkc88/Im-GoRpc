package rpc

import (
	"fmt"
	atomic "sync/atomic"
	"testing"
	"time"
)

type TestTask struct {
	I int
}

var total int32 = 5000000

func (t TestTask) Run() {
	//	fmt.Println("testTask", t.I)
	atomic.AddInt32(&total, -1)
}

func TestWorkerPool(t *testing.T) {
	pool := NewWorkerPool(100000, 220)

	i := int(total)
	for i > 0 {
		var task Task = TestTask{I: i}
		i--
		b := pool.Submit(&task)
		if !b {
			fmt.Println(b)
		}
		//		fmt.Println(b)
	}
	fmt.Println("end")

	fmt.Println(pool.GetTaskNum())
	fmt.Println(pool.GetWorkerNum())
	time.Sleep(1 * time.Second)
	fmt.Println(atomic.LoadInt32(&total))
}

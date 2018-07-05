package main

import (
	"fmt"
	Loger "github.com/lkclkc88/log4g"
	tcp "im/go/rpc"
	p "im/go/rpc/imPack"
	h "im/handler"
	"os"
	"path/filepath"
	"strings"
	atomic "sync/atomic"
	"time"
)

var log = Loger.GetLogger()

func startServer() {
	ts := tcp.NewServer()
	ts.Addr = ":18080"
	ts.HeartTime = 15
	var handler tcp.Handler = h.DefaultHandle{}
	ts.Handler = &handler
	var pack tcp.Pack = p.ImPack{}
	ts.Pack = &pack
	log.Info("------- start   server -----")
	err := ts.InitServer()
	if err != nil {
		log.Info("------- start   server failed-----")
		log.Error(err)
	} else {
		log.Info("------- start   server success-----")
		for {
			time.Sleep(1000 * time.Minute)
		}
	}
}
func substr(s string, pos, length int) string {
	runes := []rune(s)
	l := pos + length
	if l > len(runes) {
		l = len(runes)
	}
	return string(runes[pos:l])
}
func getParentDirectory(dirctory string) string {
	return substr(dirctory, 0, strings.LastIndex(dirctory, "/"))
}

func GetCurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		//		log.Error(err)
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func StartLog() {
	path := GetCurrentDirectory()
	path += "/logConfig.json"
	//	Loger.LoadConfiguration(path, "json")
	file, err := os.Open(path)
	if nil == err {
		Loger.LoadConfig(file)
		log.Info("---------init log read config " + path + "--------")
	} else {
		fmt.Println("init log file failed")
	}

}

type TestTask struct {
	I int
}

var total int32 = 5000000

func (t TestTask) Run() {
	//	fmt.Println("testTask", t.I)
	atomic.AddInt32(&total, -1)
}

/*
启动
*/
func main() {
	StartLog()
	startServer()

	//	pool := tcp.NewWorkerPool(100000, 220)
	//
	//	i := int(total)
	//	for i > 0 {
	//		var task tcp.Task = TestTask{I: i}
	//		i--
	//		b := pool.Submit(&task)
	//		if !b {
	//			fmt.Println(b)
	//		}
	//		//		fmt.Println(b)
	//	}
	//	fmt.Println("end")
	//
	//	fmt.Println(pool.GetTaskNum())
	//	fmt.Println(pool.GetWorkerNum())
	//	time.Sleep(2 * time.Second)
	//	fmt.Println(total)
	for {
		time.Sleep(24 * time.Hour)
	}
}

package test

//import (
//	"fmt"
//	//	"github.com/lkclkc88/Im-GoRpc/src/im/go/rpc"
//	Loger "github.com/lkclkc88/log4g"
//	"im/go/rpc"
//	"os"
//	"path/filepath"
//	"strings"
//	"time"
//)
//
//func StartServer() {
//	ts := rpc.NewServer()
//	ts.Addr = ":18080"
//	ts.HeartTime = 0
//	var handler rpc.Handler = DefaultHandle{}
//	ts.Handler = &handler
//	ts.MaxWorker = 150
//	ts.MaxQueueSize = 50000
//	//	ts.AsyncReadWrite = false
//	var pack rpc.Pack = rpc.ImPack{}
//	ts.Pack = &pack
//	log.Info("------- start   server -----")
//	err := ts.InitServer()
//	if err != nil {
//		log.Info("------- start   server failed-----")
//		log.Error(err)
//	} else {
//		log.Info("------- start   server success-----")
//		for {
//			time.Sleep(1000 * time.Minute)
//		}
//	}
//}
//func substr(s string, pos, length int) string {
//	runes := []rune(s)
//	l := pos + length
//	if l > len(runes) {
//		l = len(runes)
//	}
//	return string(runes[pos:l])
//}
//func getParentDirectory(dirctory string) string {
//	return substr(dirctory, 0, strings.LastIndex(dirctory, "/"))
//}
//
//func GetCurrentDirectory() string {
//	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
//	if err != nil {
//		//		log.Error(err)
//	}
//	return strings.Replace(dir, "\\", "/", -1)
//}
//
//func StartLog(path string) {
//	file, err := os.Open(path)
//	if nil == err {
//		Loger.LoadConfig(file)
//		log.Info("---------init log read config " + path + "--------")
//	} else {
//		fmt.Println("init log file failed")
//	}
//
//}

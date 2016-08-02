package main

import (
	"fmt"
	"os"
	"runtime"
	"github.com/arsgo/lib4go/forever"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	appServer, err := NewAPPServer()
	if err != nil {
		fmt.Println(err)
		os.Exit(100)
	}
	f := forever.NewForever(appServer, appServer.Log, "appserver", "appserver")
	f.Start()
}

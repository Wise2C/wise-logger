package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/golang/glog"
)

var HOST = ""

func init() {
	b, _ := ioutil.ReadFile("/etc/hostname")
	if len(b) > 0 {
		b = b[0 : len(b)-1]
	}
	HOST = string(b)
}

func main() {
	flag.Parse()

	c := make(chan ContainerChangeInfo, 1)
	go CreateConfig(c)
	go WatchLogVolume(c)
	//	go GatherLogVolumeTask(c)

	go WatchTmpl(c)
	//	go WatchEtcd(c)

	glog.Info(http.ListenAndServe("0.0.0.0:6060", nil))
}

func Recover() {
	if err := recover(); err != nil {
		const size = 4096
		buf := make([]byte, size)
		buf = buf[:runtime.Stack(buf, false)]
		glog.Errorf("panic: %v\n%s", err, buf)
		glog.Flush()
	}
}

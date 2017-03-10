package main

import (
	_ "expvar"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/hashicorp/consul/api"
)

var (
	HOST = ""
)

func init() {
	b, _ := ioutil.ReadFile("/etc/hostname")
	if len(b) > 0 {
		b = b[0 : len(b)-1]
	}
	HOST = string(b)
}

func main() {
	initGlog()
	defer glog.Flush()

	initSignal()

	c := make(chan ContainerChangeInfo, 1)

	getKafka(c)

	go CreateConfig(c)
	go WatchLogVolume(c)

	if tmplSource == "file" {
		go WatchTmplFile(c)
	} else if tmplSource == "etcd" {
		go WatchEtcd(c)
	}

	glog.Info(http.ListenAndServe("0.0.0.0:6060", nil))
}

func initSignal() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		syscall.SIGKILL,
	)

	go func() {
		sig := <-sc
		glog.Infof("receive signal [%d] to exit", sig)
		glog.Flush()
		os.Exit(0)
	}()
}

func initGlog() {
	p := flag.Lookup("log_dir").Value.String()
	if p == "" {
		p = "./log"
		flag.Set("log_dir", p)
	}

	if err := os.MkdirAll(p, 0755); err != nil {
		flag.Set("toStderr", "true")
		glog.Error(err.Error())
	}

	flag.Parse()
	glog.Infof("finish initializing glog")
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

func getKafka(c chan ContainerChangeInfo) error {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		panic(err)
	}

	kv := client.KV()

	pair, _, err := kv.Get("system/logger/kafkaip", nil)
	if err != nil {
		return err
	}

	c <- ContainerChangeInfo{
		Info: &GlobalInfo{
			Vars: map[string]string{
				"kafka": string(pair.Value),
			},
		},
		ChangeType: UPDATE,
	}

	go GetKafka(kv, pair.ModifyIndex, c)

	return nil
}

func GetKafka(kv *api.KV, i uint64, c chan ContainerChangeInfo) {
	for {
		pair, _, err := kv.Get("system/logger/kafkaip", &api.QueryOptions{
			WaitIndex: i,
			WaitTime:  5 * time.Second,
		})

		if err != nil {
			fmt.Println("error: ", err.Error())
		}

		if i == pair.ModifyIndex {
			continue
		}
		i = pair.ModifyIndex

		c <- ContainerChangeInfo{
			Info: &GlobalInfo{
				Vars: map[string]string{
					"kafka": string(pair.Value),
				},
			},
			ChangeType: UPDATE,
		}
	}
}

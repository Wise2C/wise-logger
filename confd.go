package main

import (
	"fmt"
	"os"
	"text/template"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/fsnotify-master"
	"github.com/golang/glog"
)

type ChangeType int

const (
	ADD ChangeType = iota
	RM
	CHANGE
	NONE
)

var (
	ETCD_POINT        = os.Getenv("ETCD_POINT")
	DOCKERAPI_VERSION = os.Getenv("DOCKERAPI_VERSION")

	tmpl string
)

type ContainerChangeInfo struct {
	Info       map[string]*ContainerInfo
	ChangeType ChangeType
}

func WatchEtcd(c chan<- ContainerChangeInfo) {
	defer Recover()

	cfg := client.Config{
		Endpoints: []string{ETCD_POINT},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	cl, err := client.New(cfg)
	if err != nil {
		glog.Error(err)
	}

	kapi := client.NewKeysAPI(cl)
	watch := kapi.Watcher("wiselogger",
		&client.WatcherOptions{
			AfterIndex: 0,
			Recursive:  false,
		},
	)

	for {
		res, err := watch.Next(context.Background())
		if err != nil {
			if err == context.Canceled {
				glog.Error(err.Error())
			} else if err == context.DeadlineExceeded {
				glog.Error(err.Error())
			} else if cerr, ok := err.(*client.ClusterError); ok {
				glog.Error(cerr.Error())
			} else {
				glog.Error(err.Error())
			}
		}

		c <- ContainerChangeInfo{
			Info:       nil,
			ChangeType: CHANGE,
		}

		glog.Info(res.Node.Value)
	}
}

func WatchTmpl(c chan<- ContainerChangeInfo) {
	defer Recover()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatalf("initialize fsnotify error: %s", err.Error())
	}

	err = watcher.Watch("templates/conf.tmpl")
	if err != nil {
		glog.Fatalf("watch template error: %s", err.Error())
	}

	eventIsModify := true
	for {
		select {
		case ev := <-watcher.Event:
			if ev.IsModify() {
				if !eventIsModify {
					c <- ContainerChangeInfo{
						Info:       nil,
						ChangeType: NONE,
					}
				}
				eventIsModify = !eventIsModify
				glog.Infof("template is modified")
			}
		case err := <-watcher.Error:
			glog.Errorf("error: %s", err.Error())
		}
	}
}

func CreateConfig(c <-chan ContainerChangeInfo) {
	defer Recover()

	if err := getEtcdConfig(); err != nil {
		glog.Fatal(err.Error())
	}
	cl := make(map[string]*ContainerInfo)

	for {
		select {
		case ci := <-c:
			if ci.ChangeType == ADD {
				for k, v := range ci.Info {
					cl[k] = v
				}
			} else if ci.ChangeType == RM {
				for k, _ := range ci.Info {
					delete(cl, k)
				}
			} else if ci.ChangeType == CHANGE {
				if err := getEtcdConfig(); err != nil {
					glog.Fatal(err.Error())
				}
			}
			createConfig(cl)
		}
	}
}

func getEtcdConfig() error {
	cfg := client.Config{
		Endpoints: []string{ETCD_POINT},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	ec, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("create etcd client error: %s", err.Error())
	}

	kapi := client.NewKeysAPI(ec)
	resp, err := kapi.Get(context.Background(), "wiselogger", nil)
	if err != nil {
		return fmt.Errorf("get config error: %s", err.Error())
	}

	tmpl = resp.Node.Value

	return nil
}

func createConfig(cl map[string]*ContainerInfo) {
	filename := "/tmp/conf.d/logstash.conf"
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		glog.Errorf("create config file error: %s", err.Error())
		return
	}
	defer file.Close()

	t := template.Must(template.New("log").Parse(tmpl))
	t.Execute(file, cl)
	glog.Info("create logstash profile")
}

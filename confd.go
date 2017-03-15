package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"text/template"
	"time"

	"golang.org/x/net/context"

	"github.com/coreos/etcd/client"
	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"
)

type ChangeType int

const (
	ADD ChangeType = iota
	RM
	UPDATE
	CHANGE
	NONE
)

var (
	tmplSource       = os.Getenv("TMPL_SOURCE")
	etcdPoint        = os.Getenv("ETCD_POINT")
	dockerAPIVersion = os.Getenv("DOCKERAPI_VERSION")

	kapi = newEtcdClient(etcdPoint)

	tmpl string
)

type GlobalInfo struct {
	Containers map[string]*ContainerInfo
	Vars       map[string]string
}

type ContainerChangeInfo struct {
	Info       *GlobalInfo
	ChangeType ChangeType
}

func newEtcdClient(etcdPoint string) client.KeysAPI {
	cfg := client.Config{
		Endpoints: []string{etcdPoint},
		Transport: client.DefaultTransport,
		// set timeout per request to fail fast when the target endpoint is unavailable
		HeaderTimeoutPerRequest: time.Second,
	}

	ec, err := client.New(cfg)
	if err != nil {
		panic(fmt.Sprintf("create etcd client error: %s", err.Error()))
	}

	return client.NewKeysAPI(ec)
}

func WatchEtcd(c chan<- ContainerChangeInfo) {
	defer Recover()

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

func WatchTmplFile(c chan<- ContainerChangeInfo) {
	defer Recover()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatalf("initialize fsnotify error: %s", err.Error())
	}

	err = watcher.Add("template/conf.gotmpl")
	if err != nil {
		glog.Fatalf("watch template error: %s", err.Error())
	}

	eventIsModify := true
	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&fsnotify.Write == fsnotify.Write {
				if !eventIsModify {
					c <- ContainerChangeInfo{
						Info:       nil,
						ChangeType: CHANGE,
					}
				}
				eventIsModify = !eventIsModify
				glog.Infof("template is modified")
			}
		case err := <-watcher.Errors:
			glog.Errorf("error: %s", err.Error())
		}
	}
}

func CreateConfig(c <-chan ContainerChangeInfo) {
	defer Recover()

	if err := getTmpl(); err != nil {
		glog.Fatal(err.Error())
	}

	gi := &GlobalInfo{
		Containers: make(map[string]*ContainerInfo),
		Vars: map[string]string{
			"kafkaBrokerList": "",
		},
	}

	for {
		select {
		case ci := <-c:
			if ci.ChangeType == ADD {
				for k, v := range ci.Info.Containers {
					gi.Containers[k] = v
				}
			} else if ci.ChangeType == RM {
				for k := range ci.Info.Containers {
					delete(gi.Containers, k)
				}
			} else if ci.ChangeType == UPDATE {
				for k, v := range ci.Info.Vars {
					gi.Vars[k] = v
				}
			} else if ci.ChangeType == CHANGE {
				if err := getTmpl(); err != nil {
					glog.Fatal(err.Error())
				}
			}

			if len(gi.Containers) != 0 {
				createConfig(gi)
			}
		}
	}
}

func getTmpl() error {
	if tmplSource == "etcd" {
		return getTmplFromETCD()
	} else {
		return getTmplFromFile()
	}
}

func getTmplFromETCD() error {
	resp, err := kapi.Get(context.Background(), "wiselogger", nil)
	if err != nil {
		return fmt.Errorf("get config error: %s", err.Error())
	}

	tmpl = resp.Node.Value
	return nil
}

func getTmplFromFile() error {
	filename := "template/conf.gotmpl"
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("create config file error: %s", err.Error())
	}
	defer file.Close()

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read from %s error: %s", filename, err.Error())
	}

	tmpl = string(fileContent)
	return nil
}

func createConfig(cl *GlobalInfo) {
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

package main

import (
	"os"
	"text/template"

	"github.com/fsnotify-master"
	"github.com/golang/glog"
)

type ChangeType int

const (
	ADD ChangeType = iota
	RM
	NONE
)

type ContainerChangeInfo struct {
	Info       map[string]*ContainerInfo
	ChangeType ChangeType
}

func WatchTmpl(c chan<- ContainerChangeInfo) {
	defer Recover()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatalf("initialize fsnotify error: %s", err.Error())
	}

	err = watcher.Watch("templates/conf.tmpl")
	if err != nil {
		glog.Fatalf("watch file %s error: %s", "templates/conf.tmpl", err.Error())
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
					eventIsModify = true
				} else {
					eventIsModify = false
				}
			}
		case err := <-watcher.Error:
			glog.Info("error:", err)
		}
	}
}

func CreateConfig(c <-chan ContainerChangeInfo) {
	defer Recover()

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
			}

			filename := "/home/mian/workspace/docker/logstash/logstash.conf"
			file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)
			if err != nil {
				glog.Errorf("create file %s error: ", filename, err.Error())
				return
			}
			defer file.Close()

			t := template.Must(template.ParseFiles("templates/conf.tmpl"))
			t.Execute(file, cl)
			glog.Info("create logstash profile")
		}
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/golang/glog"
)

// ContainerInfo have all fileds to generate logstash config file
type ContainerInfo struct {
	LogType     []string
	ID          string
	MountSource string
	Stack       string
	Service     string
	Index       string
	Host        string
}

func WatchLogVolume(c chan<- ContainerChangeInfo) {
	defer Recover()

	defaultHeaders := map[string]string{"User-Agent": "engine-api-client-1.0"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", dockerAPIVersion, nil, defaultHeaders)
	if err != nil {
		glog.Errorf("create docker client error: %s", err.Error())
		return
	}

	options := types.ContainerListOptions{
		All:    true,
		Filter: filters.NewArgs(),
	}
	// options.Filter.Add("label", "logtype")
	containers, err := cli.ContainerList(context.Background(), options)
	if err != nil {
		glog.Errorf("get container error: %s", err.Error())
		return
	}

	cci := make(map[string]*ContainerInfo)
	for _, c := range containers {
		info, err := getContainerInfo(cli, c.ID)
		if err != nil {
			glog.Warning(err.Error())
			continue
		}
		cci[c.ID] = info
		glog.Infof("gather log container %s: %s", c.ID, c.Names[0])
	}

	c <- ContainerChangeInfo{
		ChangeType: ADD,
		Info:       cci,
	}

	glog.Error(watchLogVolume(cli, c))
}

func watchLogVolume(cli *client.Client, c chan<- ContainerChangeInfo) error {
	ops := types.EventsOptions{
		Filters: filters.NewArgs(),
	}
	ops.Filters.Add("type", "container")
	ops.Filters.Add("event", "create")
	// ops.Filters.Add("label", "logtype")

	reader, err := cli.Events(context.Background(), ops)
	if err != nil {
		return fmt.Errorf("watch docker event error: %s", err.Error())
	}
	defer reader.Close()

	decoder := json.NewDecoder(reader)
	event := &events.Message{}
	for {
		err = decoder.Decode(event)
		if err != nil {
			return fmt.Errorf("read event error: %s", err.Error())
		}

		info, err := getContainerInfo(cli, event.ID)
		if err != nil {
			return err
		}
		c <- ContainerChangeInfo{
			Info:       map[string]*ContainerInfo{event.ID: info},
			ChangeType: ADD,
		}
	}
}

func getContainerInfo(cli *client.Client, containerID string) (*ContainerInfo, error) {
	info, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("get container info error: %s", err.Error())
	}

	if info.Name == "/rancher-agent" {
		return nil, fmt.Errorf("don't manage rancher agent")
	}

	var source string
	var lts []string
	t, ok := info.Config.Labels["logtype"]
	if ok {
		lts = strings.Split(t, ";")
		for _, m := range info.Mounts {
			if m.Name == "rancher-cni" {
				continue
			}

			p1 := filepath.Dir(m.Source)
			p1 = filepath.Dir(p1)
			source, err = filepath.Rel(p1, m.Source)
			if err != nil {
				return nil, fmt.Errorf("get container volume path error: %s", err.Error())
			}
		}
	} else {
		lts = []string{"NONE"}
	}

	var stack, service, index string
	stackService := info.Config.Labels["io.rancher.stack_service.name"]
	combination := strings.Split(stackService, "/")
	if len(combination) == 2 {
		stack = combination[0]
		service = combination[1]
	}

	containerName := info.Config.Labels["io.rancher.container.name"]
	combination = strings.Split(containerName, "-")
	if len(combination) != 0 {
		index = combination[len(combination)-1]
	}

	return &ContainerInfo{
		LogType:     lts,
		ID:          info.ID,
		MountSource: source,
		Stack:       stack,
		Service:     service,
		Index:       index,
		Host:        HOST,
	}, nil
}

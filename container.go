package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/docker/engine-api/types/events"
	"github.com/docker/engine-api/types/filters"
	"github.com/golang/glog"
)

type ContainerInfo struct {
	LogType     []string
	MountSource string
	Stack       string
	Service     string
	Index       string
	Host        string
}

func (c *ContainerInfo) String() string {
	return fmt.Sprintf("Stack: %s, Service: %s, Index: %s, MountSource: %s",
		c.Stack,
		c.Service,
		c.Index,
		c.MountSource,
	)
}

func WatchLogVolume(c chan<- ContainerChangeInfo) {
	defer Recover()

	defaultHeaders := map[string]string{"User-Agent": "wise-logger-1.0"}
	cli, err := client.NewClient("unix:///var/run/docker.sock", "v1.12", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}

	options := types.ContainerListOptions{All: true}
	containers, err := cli.ContainerList(context.Background(), options)
	if err != nil {
		panic(err)
	}

	for _, c := range containers {
		fmt.Println(c.ID)
	}
}

func watchLogVolume(cli *client.Client, c chan<- ContainerChangeInfo) error {
	ops := types.EventsOptions{
		Filters: filters.NewArgs(),
	}
	ops.Filters.Add("type", "container")
	ops.Filters.Add("event", "create")
	ops.Filters.Add("label", "logtype")

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
			ChangeType: NONE,
		}
	}

	return nil
}

func getContainerInfo(cli *client.Client, containerID string) (*ContainerInfo, error) {
	info, err := cli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return nil, fmt.Errorf("get container info error: %s", err.Error())
	}

	t, ok := info.Config.Labels["logtype"]
	if !ok {
		panic(fmt.Sprintf("recive a container %s without logtype label", containerID))
	}

	var source string
	for _, m := range info.Mounts {
		if m.Driver == "local" {
			p1 := filepath.Dir(m.Source)
			p1 = filepath.Dir(p1)
			source, err = filepath.Rel(p1, m.Source)
			if err != nil {
				return nil, fmt.Errorf("get container volume path error: %s", err.Error())
			}
		}
	}

	containerName := info.Config.Labels["io.rancher.container.name"]
	var stack, service, index string
	combination := strings.Split(containerName, "_")
	if len(combination) == 3 {
		stack = combination[0]
		service = combination[1]
		index = combination[2]
	}

	return &ContainerInfo{
		LogType:     strings.Split(t, ";"),
		MountSource: source,
		Stack:       stack,
		Service:     service,
		Index:       index,
		Host:        HOST,
	}, nil

}

func GatherLogVolumeTask(c chan<- ContainerChangeInfo) {
	defer Recover()

	var latestContainer = new(string)
	*latestContainer = ""
	GatherLogVolume(latestContainer, c)

	for {
		select {
		case <-time.After(15 * time.Second):
			GatherLogVolume(latestContainer, c)
		}
	}
}

func GatherLogVolume(latestContainer *string, c chan<- ContainerChangeInfo) {
	cs, err := GetLogContainers(*latestContainer)
	if err != nil {
		glog.Error(err.Error())
		return
	}

	if cs == nil {
		glog.Info("gather none")
		return
	}
	*latestContainer = cs[0]

	csi, err := GetContainersInfo(cs)
	if err != nil {
		glog.Error(err.Error())
	}
	glog.Infof("gather log container: %v", csi)

	c <- ContainerChangeInfo{
		ChangeType: ADD,
		Info:       csi,
	}
}

func GetLogContainers(latestContainer string) ([]string, error) {
	var cmd *exec.Cmd
	if latestContainer == "" {
		cmd = exec.Command("docker", "ps", "-a", "-q", "-f label=logtype")
	} else {
		cmd = exec.Command("docker", "ps", "-a", "-q", "-f label=logtype", fmt.Sprintf("-f since=%s", latestContainer))
	}

	buf, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get docker log containers error: %s with cmd %v", err.Error(), cmd.Args)
	}

	if len(buf) == 0 {
		return nil, nil
	}

	return strings.Split(string(buf[0:len(buf)-1]), "\n"), nil
}

func GetContainersInfo(containersID []string) (map[string]*ContainerInfo, error) {
	cci := make(map[string]*ContainerInfo)

	for _, id := range containersID {
		info, err := GetContainerInfo(id)
		if err != nil {
			return nil, err
		}
		cci[id] = info
	}

	return cci, nil
}

func GetContainerInfo(containerID string) (*ContainerInfo, error) {
	containerDetailCmd := exec.Command(
		"docker",
		"inspect",
		"-f",
		`{{ index .Config.Labels "logtype" }}|{{ (index .Mounts 0).Source }}|{{ index .Config.Labels "io.rancher.stack.name" }}|{{ index .Config.Labels "io.rancher.service.name" }}|{{ index .Config.Labels "io.rancher.create.index" }}`,
		containerID,
	)

	buf, err := containerDetailCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("get container %s info error: %s", containerID, err.Error())
	}

	info := strings.Split(string(buf[0:len(buf)-1]), "|")

	p1 := filepath.Dir(info[1])
	p1 = filepath.Dir(p1)
	p2, err := filepath.Rel(p1, info[1])
	if err != nil {
		return nil, fmt.Errorf("get container volume path error: %s", err.Error())
	}

	return &ContainerInfo{
		LogType:     strings.Split(info[0], ";"),
		MountSource: p2,
		Stack:       info[2],
		Service:     info[3],
		Index:       info[4],
		Host:        HOST,
	}, nil
}

func CheckContainer(containerID string) (bool, error) {
	cmd := exec.Command("docker", "ps", "-q", fmt.Sprintf("-f id=%s", containerID))

	buf, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("check container %s error: %s", containerID, err.Error())
	}

	if len(buf) != 0 {
		return true, nil
	} else {
		return false, nil
	}
}

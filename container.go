package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
		return nil, fmt.Errorf("get docker log containers error: %s since - %s", err.Error(), latestContainer)
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

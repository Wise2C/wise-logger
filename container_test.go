package main

import (
	"testing"
)

func TestGetLogContainers(t *testing.T) {
	latestContainer := ""

	cs, err := GetLogContainers(latestContainer)
	if err != nil {
		t.Errorf("get contaners error: %s", err.Error())
	}

	t.Log(cs)

	cs, err = GetLogContainers(cs[0])
	if err != nil {
		t.Errorf("get container error: %s", err.Error())
	}

	if len(cs) != 0 {
		t.Error("此时应该不会获取任何容器")
	}
}

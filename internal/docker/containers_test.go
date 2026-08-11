package docker

import "testing"

func TestShouldCollectHonorsCollectLabel(t *testing.T) {
	if ShouldCollect(Container{Labels: map[string]string{CollectLabel: "false"}}) {
		t.Fatal("container with caroline.collect=false should be skipped")
	}
	if !ShouldCollect(Container{Labels: map[string]string{CollectLabel: "true"}}) {
		t.Fatal("container with caroline.collect=true should be collected")
	}
	if !ShouldCollect(Container{}) {
		t.Fatal("container without collect label should be collected")
	}
}

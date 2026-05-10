package main

import "testing"

func TestStoreReplaceAll_DropsRemovedContainers(t *testing.T) {
	s := NewStore()
	s.ReplaceAll(map[string]AppState{
		"oldid": {ContainerID: "oldid", ContainerName: "app"},
	})
	s.ReplaceAll(map[string]AppState{
		"newid": {ContainerID: "newid", ContainerName: "app"},
	})
	list := s.List()
	if len(list) != 1 {
		t.Fatalf("want 1 app, got %d", len(list))
	}
	if list[0].ContainerID != "newid" {
		t.Fatalf("want newid, got %q", list[0].ContainerID)
	}
}

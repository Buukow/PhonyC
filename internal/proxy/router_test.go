package proxy

import (
	"testing"

	"github.com/phonyg/phonyg/internal/snapshot"
	"github.com/phonyg/phonyg/internal/store"
)

func TestSelectChannelPriority(t *testing.T) {
	snap := &snapshot.Snapshot{
		ModelsByClient: map[string][]snapshot.ModelCandidate{
			"m1": {
				{Channel: store.Channel{ID: 1, Protocol: "openai", Enabled: true, Priority: 1}, Model: store.ChannelModel{ClientModel: "m1", Enabled: true}},
				{Channel: store.Channel{ID: 2, Protocol: "openai", Enabled: true, Priority: 10}, Model: store.ChannelModel{ClientModel: "m1", Enabled: true}},
				{Channel: store.Channel{ID: 3, Protocol: "anthropic", Enabled: true, Priority: 100}, Model: store.ChannelModel{ClientModel: "m1", Enabled: true}},
			},
		},
	}
	c, ok := SelectChannel(snap, "openai", "m1")
	if !ok || c.Channel.ID != 2 {
		t.Fatalf("got=%+v ok=%v", c, ok)
	}
}

func TestSelectChannelRandomSamePriority(t *testing.T) {
	snap := &snapshot.Snapshot{
		ModelsByClient: map[string][]snapshot.ModelCandidate{
			"m1": {
				{Channel: store.Channel{ID: 1, Protocol: "openai", Enabled: true, Priority: 5}, Model: store.ChannelModel{Enabled: true}},
				{Channel: store.Channel{ID: 2, Protocol: "openai", Enabled: true, Priority: 5}, Model: store.ChannelModel{Enabled: true}},
			},
		},
	}
	seen := map[int64]bool{}
	for i := 0; i < 50; i++ {
		c, ok := SelectChannel(snap, "openai", "m1")
		if !ok {
			t.Fatal("expected ok")
		}
		seen[c.Channel.ID] = true
	}
	if len(seen) < 2 {
		t.Fatalf("expected both channels over 50 picks, got %v", seen)
	}
}

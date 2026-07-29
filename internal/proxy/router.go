package proxy

import (
	"math/rand"
	"sort"

	"github.com/phonyc/phonyc/internal/snapshot"
)

type RouteResult struct {
	Candidate snapshot.ModelCandidate
}

// SelectChannel picks among enabled candidates matching protocol+clientModel.
// Highest priority tier wins; within tier, random.
func SelectChannel(snap *snapshot.Snapshot, protocol, clientModel string) (snapshot.ModelCandidate, bool) {
	return SelectChannelExcluding(snap, protocol, clientModel, nil)
}

// SelectChannelExcluding is SelectChannel but skips channel IDs in exclude.
func SelectChannelExcluding(snap *snapshot.Snapshot, protocol, clientModel string, exclude map[int64]struct{}) (snapshot.ModelCandidate, bool) {
	if snap == nil {
		return snapshot.ModelCandidate{}, false
	}
	cands := snap.ModelsByClient[clientModel]
	var matched []snapshot.ModelCandidate
	for _, c := range cands {
		if c.Channel.Protocol != protocol {
			continue
		}
		if !c.Channel.Routable() || !c.Model.Enabled {
			continue
		}
		if exclude != nil {
			if _, skip := exclude[c.Channel.ID]; skip {
				continue
			}
		}
		matched = append(matched, c)
	}
	if len(matched) == 0 {
		return snapshot.ModelCandidate{}, false
	}
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].Channel.Priority > matched[j].Channel.Priority
	})
	top := matched[0].Channel.Priority
	var tier []snapshot.ModelCandidate
	for _, c := range matched {
		if c.Channel.Priority == top {
			tier = append(tier, c)
		} else {
			break
		}
	}
	if len(tier) == 1 {
		return tier[0], true
	}
	return tier[rand.Intn(len(tier))], true
}

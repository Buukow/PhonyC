package snapshot

import (
	"sort"
	"sync/atomic"

	"github.com/phonyg/phonyg/internal/store"
)

type Snapshot struct {
	Version       uint64
	Channels      []store.Channel
	ChannelModels []store.ChannelModel
	UserKeys      []store.UserKey
	Presets       []store.ClientPreset
	// indexes
	KeyByValue      map[string]*store.UserKey
	ChannelByID     map[int64]*store.Channel
	ModelsByChannel map[int64][]store.ChannelModel
	PresetByID      map[int64]*store.ClientPreset
	// client_model -> candidates
	// filtered at route time by protocol
	ModelsByClient map[string][]ModelCandidate
	// unique client models for catalog
	ClientModels []string
}

type ModelCandidate struct {
	Channel store.Channel
	Model   store.ChannelModel
}

type Manager struct {
	ptr atomic.Pointer[Snapshot]
	st  *store.Store
	ver atomic.Uint64
}

func NewManager(st *store.Store) *Manager {
	m := &Manager{st: st}
	return m
}

func (m *Manager) Current() *Snapshot {
	return m.ptr.Load()
}

func (m *Manager) Reload() error {
	channels, err := m.st.ListChannels()
	if err != nil {
		return err
	}
	models, err := m.st.ListAllChannelModels()
	if err != nil {
		return err
	}
	keys, err := m.st.ListUserKeys()
	if err != nil {
		return err
	}
	presets, err := m.st.ListPresets()
	if err != nil {
		return err
	}
	snap := &Snapshot{
		Channels:        channels,
		ChannelModels:   models,
		UserKeys:        keys,
		Presets:         presets,
		KeyByValue:      map[string]*store.UserKey{},
		ChannelByID:     map[int64]*store.Channel{},
		ModelsByChannel: map[int64][]store.ChannelModel{},
		PresetByID:      map[int64]*store.ClientPreset{},
		ModelsByClient:  map[string][]ModelCandidate{},
	}
	for i := range channels {
		c := &channels[i]
		snap.ChannelByID[c.ID] = c
	}
	for i := range presets {
		p := &presets[i]
		snap.PresetByID[p.ID] = p
	}
	for i := range keys {
		k := &keys[i]
		snap.KeyByValue[k.Key] = k
	}
	seenModels := map[string]struct{}{}
	for i := range models {
		cm := models[i]
		snap.ModelsByChannel[cm.ChannelID] = append(snap.ModelsByChannel[cm.ChannelID], cm)
		ch, ok := snap.ChannelByID[cm.ChannelID]
		if !ok || !ch.Routable() || !cm.Enabled {
			continue
		}
		cand := ModelCandidate{Channel: *ch, Model: cm}
		snap.ModelsByClient[cm.ClientModel] = append(snap.ModelsByClient[cm.ClientModel], cand)
		if _, ok := seenModels[cm.ClientModel]; !ok {
			seenModels[cm.ClientModel] = struct{}{}
			snap.ClientModels = append(snap.ClientModels, cm.ClientModel)
		}
	}
	sort.Strings(snap.ClientModels)
	snap.Version = m.ver.Add(1)
	m.ptr.Store(snap)
	return nil
}

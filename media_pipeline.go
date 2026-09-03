package main

import (
	"errors"
	"sync"
)

type AssetState string

const (
	AssetIngested   AssetState = "ingested"
	AssetProcessing AssetState = "processing"
	AssetReady      AssetState = "ready"
	AssetDelivered  AssetState = "delivered"
)

type Asset struct {
	ID        string     `json:"id"`
	CreatorID string     `json:"creator_id"`
	Source    string     `json:"source"`
	State     AssetState `json:"state"`
}

type AssetStore struct {
	mu     sync.Mutex
	assets map[string]Asset
}

func NewAssetStore() *AssetStore {
	return &AssetStore{assets: make(map[string]Asset)}
}

func (s *AssetStore) Ingest(asset Asset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if asset.ID == "" || asset.CreatorID == "" || asset.Source == "" {
		return errors.New("id, creator_id, and source are required")
	}
	if _, exists := s.assets[asset.ID]; exists {
		return errors.New("asset already exists")
	}
	asset.State = AssetIngested
	s.assets[asset.ID] = asset
	return nil
}

func (s *AssetStore) Advance(id string, next AssetState) (Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset, ok := s.assets[id]
	if !ok {
		return Asset{}, errors.New("asset not found")
	}
	allowed := (asset.State == AssetIngested && next == AssetProcessing) ||
		(asset.State == AssetProcessing && next == AssetReady)
	if !allowed {
		return Asset{}, errors.New("invalid asset transition")
	}
	asset.State = next
	s.assets[id] = asset
	return asset, nil
}

func (s *AssetStore) Deliver(id, creatorID string) (Asset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	asset, ok := s.assets[id]
	if !ok {
		return Asset{}, errors.New("asset not found")
	}
	if asset.CreatorID != creatorID {
		return Asset{}, errors.New("creator does not own asset")
	}
	if asset.State != AssetReady {
		return Asset{}, errors.New("asset is not ready")
	}
	asset.State = AssetDelivered
	s.assets[id] = asset
	return asset, nil
}

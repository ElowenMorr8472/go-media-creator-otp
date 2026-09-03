package main

import "testing"

func TestAssetDeliveryDecision(t *testing.T) {
	tests := []struct {
		name      string
		advance   []AssetState
		creatorID string
		wantState AssetState
		wantErr   bool
	}{
		{name: "ready asset reaches its creator", advance: []AssetState{AssetProcessing, AssetReady}, creatorID: "creator-7", wantState: AssetDelivered},
		{name: "processing asset stays queued", advance: []AssetState{AssetProcessing}, creatorID: "creator-7", wantErr: true},
		{name: "different creator cannot collect", advance: []AssetState{AssetProcessing, AssetReady}, creatorID: "creator-8", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewAssetStore()
			if err := store.Ingest(Asset{ID: "asset-42", CreatorID: "creator-7", Source: "s3://studio/raw/episode.mov"}); err != nil {
				t.Fatal(err)
			}
			for _, state := range tt.advance {
				if _, err := store.Advance("asset-42", state); err != nil {
					t.Fatal(err)
				}
			}
			got, err := store.Deliver("asset-42", tt.creatorID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Deliver() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got.State != tt.wantState {
				t.Fatalf("state = %q, want %q", got.State, tt.wantState)
			}
		})
	}
}

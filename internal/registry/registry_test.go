package registry

import (
	"encoding/json"
	"testing"
)

func TestManifestContentSize(t *testing.T) {
	var manifest registryManifest
	if err := json.Unmarshal([]byte(`{
		"config": {"size": 128},
		"layers": [{"size": 256}, {"size": 512}]
	}`), &manifest); err != nil {
		t.Fatal(err)
	}
	if got := manifestContentSize(manifest); got != 896 {
		t.Fatalf("content size = %d, want 896", got)
	}
}

func TestManifestContentSizeLeavesIndexUnknown(t *testing.T) {
	var manifest registryManifest
	if err := json.Unmarshal([]byte(`{
		"manifests": [{"digest": "sha256:child", "size": 1234}]
	}`), &manifest); err != nil {
		t.Fatal(err)
	}
	if got := manifestContentSize(manifest); got != 0 {
		t.Fatalf("index content size = %d, want unknown", got)
	}
}

package runtime

import (
	"reflect"
	"testing"
)

func TestEndpointConfigurationPreservesStableAliases(t *testing.T) {
	var container platformContainerInspect
	container.ID = "1234567890abcdef"
	container.NetworkSettings.Networks = map[string]struct {
		Aliases []string `json:"Aliases"`
	}{
		"dokyr_control":  {Aliases: []string{"1234567890ab", "dokyr-selfhost-1", "selfhost"}},
		"selfhost-proxy": {Aliases: []string{"selfhost"}},
	}
	got := endpointConfiguration(container)
	control := got["dokyr_control"].(map[string]any)["Aliases"]
	if !reflect.DeepEqual(control, []string{"dokyr-selfhost-1", "selfhost"}) {
		t.Fatalf("control aliases = %#v", control)
	}
	proxy := got["selfhost-proxy"].(map[string]any)["Aliases"]
	if !reflect.DeepEqual(proxy, []string{"selfhost"}) {
		t.Fatalf("proxy aliases = %#v", proxy)
	}
}

func TestImageRepositoryRemovesTagOrDigest(t *testing.T) {
	tests := map[string]string{
		"ghcr.io/azayr/dokyr:latest":     "ghcr.io/azayr/dokyr",
		"ghcr.io/azayr/dokyr@sha256:abc": "ghcr.io/azayr/dokyr",
		"registry.test:5000/dokyr:1.4.0": "registry.test:5000/dokyr",
		"registry.test:5000/dokyr":       "registry.test:5000/dokyr",
	}
	for input, want := range tests {
		if got := imageRepository(input); got != want {
			t.Errorf("imageRepository(%q) = %q, want %q", input, got, want)
		}
	}
}

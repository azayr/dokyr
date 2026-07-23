package api

import "testing"

func TestDevelopmentVersionsDoNotOfferSelfUpdate(t *testing.T) {
	for _, value := range []string{"dev", "Development", "0.1.0-dev", " 1.0.0-DEV "} {
		if !isDevelopmentVersion(value) {
			t.Errorf("isDevelopmentVersion(%q) = false", value)
		}
	}
	for _, value := range []string{"0.1.0", "1.0.0-rc.1", "sha-abcdef1"} {
		if isDevelopmentVersion(value) {
			t.Errorf("isDevelopmentVersion(%q) = true", value)
		}
	}
}

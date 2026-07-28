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

func TestOnlyNewerSemanticVersionsAreOffered(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		candidate string
		want      bool
	}{
		{name: "patch update", current: "0.2.11", candidate: "0.2.12", want: true},
		{name: "optional v prefix", current: "v0.2.11", candidate: "v0.2.12", want: true},
		{name: "minor update", current: "0.2.12", candidate: "0.3.0", want: true},
		{name: "downgrade", current: "0.2.11", candidate: "0.1.0", want: false},
		{name: "same release", current: "0.2.12", candidate: "0.2.12", want: false},
		{name: "release beats prerelease", current: "1.0.0-rc.1", candidate: "1.0.0", want: true},
		{name: "older prerelease", current: "1.0.0-rc.2", candidate: "1.0.0-rc.1", want: false},
		{name: "large prerelease number", current: "1.0.0-rc.18446744073709551616", candidate: "1.0.0-rc.18446744073709551617", want: true},
		{name: "large core number", current: "18446744073709551616.0.0", candidate: "18446744073709551617.0.0", want: true},
		{name: "build metadata ignored", current: "1.0.0+build.1", candidate: "1.0.0+build.2", want: false},
		{name: "invalid build metadata", current: "1.0.0", candidate: "1.0.1+build!", want: false},
		{name: "invalid registry label", current: "0.2.12", candidate: "latest", want: false},
		{name: "invalid current build", current: "sha-abcdef1", candidate: "0.2.12", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isNewerSemanticVersion(test.current, test.candidate); got != test.want {
				t.Fatalf("isNewerSemanticVersion(%q, %q) = %t, want %t", test.current, test.candidate, got, test.want)
			}
		})
	}
}

func TestSemanticVersionPrecedenceMatchesSpecification(t *testing.T) {
	ordered := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	for index := 1; index < len(ordered); index++ {
		if !isNewerSemanticVersion(ordered[index-1], ordered[index]) {
			t.Fatalf("%q should be newer than %q", ordered[index], ordered[index-1])
		}
	}
}

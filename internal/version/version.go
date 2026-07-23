package version

import "strings"

// These values are replaced by the release build. Development builds keep
// useful, explicit values instead of pretending to be a published release.
var (
	Version   = "0.1.0-dev"
	Revision  = "unknown"
	BuildDate = "unknown"
)

type Info struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	BuildDate string `json:"buildDate"`
}

func Current() Info {
	return Info{
		Version:   strings.TrimSpace(Version),
		Revision:  strings.TrimSpace(Revision),
		BuildDate: strings.TrimSpace(BuildDate),
	}
}

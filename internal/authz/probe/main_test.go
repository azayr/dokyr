package probe

import (
	"fmt"
	"os"
	"testing"

	"github.com/azayr/selfhost/internal/auth"
)

func TestMintProbeToken(t *testing.T) {
	if os.Getenv("PROBE") == "" {
		t.Skip("set PROBE to mint a token")
	}
	m, err := auth.New("development-secret-change-before-production", "selfhost", false)
	if err != nil {
		t.Fatal(err)
	}
	token, err := m.Token("usr_probe_owner", "Probe owner", "probe-owner@example.com", "owner")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("TOKEN=%s\n", token)
}

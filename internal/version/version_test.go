package version

import (
	"os"
	"strings"
	"testing"
)

func TestVersionMatchesFile(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("version.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(data))
	if Version != want {
		t.Errorf("Version = %q, want %q (from version.txt)", Version, want)
	}
}

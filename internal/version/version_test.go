package version

import (
	"regexp"
	"testing"
)

func TestVersionIsValidSemver(t *testing.T) {
	t.Parallel()
	if Version == "" {
		t.Fatal("Version is empty")
	}
	matched, err := regexp.MatchString(`^\d+\.\d+\.\d+$`, Version)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Errorf("Version = %q, want semver format (X.Y.Z)", Version)
	}
}

func TestVersionHasNoWhitespace(t *testing.T) {
	t.Parallel()
	if Version != Version {
		t.Errorf("Version contains whitespace: %q", Version)
	}
	if len(Version) == 0 {
		t.Fatal("Version is empty after trimming")
	}
}

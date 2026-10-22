package buildinfo

import "testing"

func TestApp_ReturnsInjectedVersion(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = "v9.9.9"
	if got := App(); got != "v9.9.9" {
		t.Errorf("App() = %q, want %q", got, "v9.9.9")
	}
}

func TestApp_FallsBackToBuildMetadata(t *testing.T) {
	orig := version
	t.Cleanup(func() { version = orig })

	version = ""
	if got := App(); got == "" {
		t.Error("App() returned empty string, want build metadata version or \"devel\"")
	}
}

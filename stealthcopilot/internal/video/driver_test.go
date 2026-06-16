package video

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDefaultBundledDriverPath_EnvOverride(t *testing.T) {
	t.Setenv("STEALTHCOPILOT_VCAM_DRIVER", "/tmp/custom-driver")
	if got := DefaultBundledDriverPath(); got != "/tmp/custom-driver" {
		t.Fatalf("DefaultBundledDriverPath() = %q, want env override", got)
	}
}

func TestMacAppResourcesDir(t *testing.T) {
	exe := filepath.Join("/", "Applications", "StealthCopilot.app", "Contents", "MacOS", "stealthcopilot")
	got := macAppResourcesDir(exe)
	want := filepath.Join("/", "Applications", "StealthCopilot.app", "Contents", "Resources")
	if got != want {
		t.Fatalf("macAppResourcesDir() = %q, want %q", got, want)
	}
}

func TestDefaultBundledDriverPath_HasPlatformArtifactName(t *testing.T) {
	t.Setenv("STEALTHCOPILOT_VCAM_DRIVER", "")
	got := DefaultBundledDriverPath()
	switch runtime.GOOS {
	case "darwin":
		if filepath.Base(got) != darwinDriverBundleName {
			t.Fatalf("driver path = %q, want %q suffix", got, darwinDriverBundleName)
		}
	case "windows":
		if filepath.Base(got) != windowsDriverDLLName {
			t.Fatalf("driver path = %q, want %q suffix", got, windowsDriverDLLName)
		}
	default:
		if got != "" {
			t.Fatalf("driver path = %q, want empty on unsupported platform", got)
		}
	}
}

func TestEnsureDriver_UsesEnvOverrideWhenMissingPath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing-driver")
	t.Setenv("STEALTHCOPILOT_VCAM_DRIVER", missing)
	result := EnsureDriver("")
	if result.Status == DriverStatusRegistered {
		t.Skip("driver already registered on this machine")
	}
	if result.Message == "" {
		t.Fatal("EnsureDriver should return a message for missing bundled driver")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("test setup expected missing driver, stat err = %v", err)
	}
}

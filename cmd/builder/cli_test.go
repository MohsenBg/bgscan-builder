package main

import (
	"testing"

	"bgscan-builder/internal/platform"
)

func TestResolvePlatforms_SpecificOSArch(t *testing.T) {
	builds := resolvePlatforms("linux", "amd64")
	if len(builds) != 1 {
		t.Fatalf("expected 1 build, got %d", len(builds))
	}
	if builds[0].OS != platform.Linux || builds[0].Arch != platform.AMD64 {
		t.Errorf("got %s, want linux-amd64", builds[0])
	}
}

func TestResolvePlatforms_AllArch(t *testing.T) {
	builds := resolvePlatforms("linux", "all")
	if len(builds) == 0 {
		t.Fatal("expected builds for linux-all")
	}
	for _, b := range builds {
		if b.OS != platform.Linux {
			t.Errorf("expected Linux, got %s", b.OS)
		}
	}
}

func TestResolvePlatforms_AllOS(t *testing.T) {
	builds := resolvePlatforms("all", "arm64")
	if len(builds) == 0 {
		t.Fatal("expected builds for all-arm64")
	}
	for _, b := range builds {
		if b.Arch != platform.ARM64 {
			t.Errorf("expected ARM64, got %s", b.Arch)
		}
	}
}

func TestResolvePlatforms_AllAll(t *testing.T) {
	builds := resolvePlatforms("all", "all")
	if len(builds) == 0 {
		t.Fatal("expected all builds")
	}
}

func TestRequiresAndroidNDK_WithAndroid(t *testing.T) {
	platforms := []platform.Info{
		{OS: platform.Linux, Arch: platform.AMD64},
		{OS: platform.Android, Arch: platform.ARM64},
	}
	if !requiresAndroidNDK(platforms) {
		t.Error("expected true for Android platform")
	}
}

func TestRequiresAndroidNDK_WithoutAndroid(t *testing.T) {
	platforms := []platform.Info{
		{OS: platform.Linux, Arch: platform.AMD64},
	}
	if requiresAndroidNDK(platforms) {
		t.Error("expected false for non-Android platforms")
	}
}

func TestRequiresAndroidNDK_Empty(t *testing.T) {
	if requiresAndroidNDK(nil) {
		t.Error("expected false for nil platforms")
	}
}

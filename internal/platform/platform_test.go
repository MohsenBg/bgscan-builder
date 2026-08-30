package platform

import "testing"

func TestOS_String(t *testing.T) {
	tests := []struct {
		os   OS
		want string
	}{
		{Linux, "linux"},
		{Android, "android"},
		{MacOS, "macos"},
		{Windows, "windows"},
		{UnknownOS, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.os.String(); got != tt.want {
			t.Errorf("OS(%d).String() = %q, want %q", int(tt.os), got, tt.want)
		}
	}
}

func TestOS_GOOS(t *testing.T) {
	tests := []struct {
		os   OS
		want string
	}{
		{Linux, "linux"},
		{Android, "android"},
		{MacOS, "darwin"},
		{Windows, "windows"},
	}
	for _, tt := range tests {
		if got := tt.os.GOOS(); got != tt.want {
			t.Errorf("OS(%d).GOOS() = %q, want %q", int(tt.os), got, tt.want)
		}
	}
}

func TestArch_String(t *testing.T) {
	tests := []struct {
		arch Arch
		want string
	}{
		{ARM64, "arm64"},
		{ARM32, "arm32"},
		{AMD64, "amd64"},
		{AMD32, "amd32"},
		{UnknownArch, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.arch.String(); got != tt.want {
			t.Errorf("Arch(%d).String() = %q, want %q", int(tt.arch), got, tt.want)
		}
	}
}

func TestArch_GOARCH(t *testing.T) {
	tests := []struct {
		arch Arch
		want string
	}{
		{ARM64, "arm64"},
		{ARM32, "arm"},
		{AMD64, "amd64"},
		{AMD32, "386"},
	}
	for _, tt := range tests {
		if got := tt.arch.GOARCH(); got != tt.want {
			t.Errorf("Arch(%d).GOARCH() = %q, want %q", int(tt.arch), got, tt.want)
		}
	}
}

func TestArch_Tokens(t *testing.T) {
	tests := []struct {
		arch Arch
		want []string
	}{
		{ARM64, []string{"arm64", "aarch64", "armv8"}},
		{ARM32, []string{"armv7", "arm32", "armeabi", "v7a"}},
		{AMD64, []string{"x86_64", "amd64", "64"}},
		{AMD32, []string{"x86", "i386", "386", "32"}},
	}
	for _, tt := range tests {
		got := tt.arch.Tokens()
		if len(got) != len(tt.want) {
			t.Errorf("Arch(%d).Tokens() len = %d, want %d", int(tt.arch), len(got), len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Arch(%d).Tokens()[%d] = %q, want %q", int(tt.arch), i, got[i], tt.want[i])
			}
		}
	}
}

func TestInfo_String(t *testing.T) {
	info := Info{OS: Linux, Arch: AMD64}
	if got := info.String(); got != "linux-amd64" {
		t.Errorf("Info.String() = %q, want %q", got, "linux-amd64")
	}
}

func TestParseOS(t *testing.T) {
	tests := []struct {
		input string
		want  OS
	}{
		{"linux", Linux},
		{"android", Android},
		{"macos", MacOS},
		{"darwin", MacOS},
		{"windows", Windows},
		{"LINUX", Linux},
		{"  Linux  ", Linux},
		{"unknown", UnknownOS},
	}
	for _, tt := range tests {
		if got := ParseOS(tt.input); got != tt.want {
			t.Errorf("ParseOS(%q) = %d, want %d", tt.input, int(got), int(tt.want))
		}
	}
}

func TestParseArch(t *testing.T) {
	tests := []struct {
		input string
		want  Arch
	}{
		{"arm64", ARM64},
		{"aarch64", ARM64},
		{"armv8", ARM64},
		{"armv7", ARM32},
		{"arm32", ARM32},
		{"arm", ARM32},
		{"amd64", AMD64},
		{"x86_64", AMD64},
		{"64", AMD64},
		{"amd32", AMD32},
		{"x86", AMD32},
		{"i386", AMD32},
		{"386", AMD32},
		{"32", AMD32},
		{"unknown", UnknownArch},
	}
	for _, tt := range tests {
		if got := ParseArch(tt.input); got != tt.want {
			t.Errorf("ParseArch(%q) = %d, want %d", tt.input, int(got), int(tt.want))
		}
	}
}

func TestGetAllBuilds(t *testing.T) {
	builds := GetAllBuilds()
	if len(builds) == 0 {
		t.Fatal("GetAllBuilds() returned empty")
	}

	seen := make(map[Info]bool)
	for _, b := range builds {
		if seen[b] {
			t.Errorf("duplicate build: %s", b.String())
		}
		seen[b] = true
	}
}

func TestGetPlatformSpecificArch(t *testing.T) {
	linuxBuilds := GetPlatformSpecificArch(Linux)
	for _, b := range linuxBuilds {
		if b.OS != Linux {
			t.Errorf("expected Linux, got %s", b.OS)
		}
	}

	androidBuilds := GetPlatformSpecificArch(Android)
	for _, b := range androidBuilds {
		if b.OS != Android {
			t.Errorf("expected Android, got %s", b.OS)
		}
	}
}

func TestDetect(t *testing.T) {
	info := Detect()
	if info.OS == UnknownOS {
		t.Error("Detect() returned UnknownOS")
	}
	if info.Arch == UnknownArch {
		t.Error("Detect() returned UnknownArch")
	}
}

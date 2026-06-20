package platform

import (
	"fmt"
)

// OS represents a strongly-typed integer enumeration for supported operating systems.
type OS int

const (
	Linux OS = iota
	Android
	MacOS
	Windows
	UnknownOS
)

// String implements the fmt.Stringer interface for standard runtime logging.
func (o OS) String() string {
	switch o {
	case Linux:
		return "linux"
	case Android:
		return "android"
	case MacOS:
		return "macos"
	case Windows:
		return "windows"
	default:
		return "unknown"
	}
}

// GOOS returns the exact system identifier string expected by the go toolchain environment variable.
func (o OS) GOOS() string {
	switch o {
	case MacOS:
		return "darwin"
	default:
		return o.String()
	}
}

// Arch represents a strongly-typed integer enumeration for supported CPU architectures.
type Arch int

const (
	ARM64 Arch = iota
	ARM32
	AMD64
	AMD32
	UnknownArch
)

// String implements the fmt.Stringer interface for standard runtime logging.
func (a Arch) String() string {
	switch a {
	case ARM64:
		return "arm64"
	case ARM32:
		return "arm32"
	case AMD64:
		return "amd64"
	case AMD32:
		return "amd32"
	default:
		return "unknown"
	}
}

// GOARCH returns the exact system layout string expected by the go toolchain environment variable.
func (a Arch) GOARCH() string {
	switch a {
	case ARM32:
		return "arm"
	case AMD32:
		return "386"
	default:
		return a.String()
	}
}

// Info aggregates target platform details into a single descriptive configuration unit.
type Info struct {
	OS   OS
	Arch Arch
}

// String provides a unified string signature representation of the system runtime metrics.
func (i Info) String() string {
	return fmt.Sprintf("%s-%s", i.OS.String(), i.Arch.String())
}


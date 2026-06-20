package compiler

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"bgscan-builder/internal/platform"
)

// MinGoVersion defines the minimum toolchain version required to execute builds.
const MinGoVersion = "1.26.3"

// Build compiles bgscan for the requested target platform and stages the
// resulting binaries and configurations into the destination directory.
func Build(target platform.Info, dest, ndkDir string) error {
	version, err := checkGoVersion()
	if err != nil {
		return err
	}

	if !isGoVersionSupported(version, MinGoVersion) {
		return fmt.Errorf("Go %s or newer is required", MinGoVersion)
	}

	workDir, err := os.MkdirTemp("", "bgscan-*")
	if err != nil {
		return fmt.Errorf("create temporary workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	if err := CloneProject(workDir); err != nil {
		return err
	}

	if err := PrepareProjectFiles(workDir, dest); err != nil {
		return fmt.Errorf("copy settings: %w", err)
	}

	if err := CopyAssets(workDir, dest); err != nil {
		return fmt.Errorf("copy assets: %w", err)
	}

	env, err := buildEnvironment(target, ndkDir)
	if err != nil {
		return err
	}

	outputName := "bgscan"
	if target.OS == platform.Windows {
		outputName += ".exe"
	}

	fmt.Printf("Compiling target target: %s/%s\n", target.OS.String(), target.Arch.String())

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workDir
	tidyCmd.Env = env
	tidyCmd.Stdout = os.Stdout
	tidyCmd.Stderr = os.Stderr
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-o", outputName, "./cmd/bgscan")
	buildCmd.Dir = workDir
	buildCmd.Env = env
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w", err)
	}

	if err := os.MkdirAll(dest, 0755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	srcBinary := filepath.Join(workDir, outputName)
	dstBinary := filepath.Join(dest, outputName)

	if err := moveArtifact(srcBinary, dstBinary); err != nil {
		return err
	}

	return nil
}

func buildEnvironment(target platform.Info, ndkDir string) ([]string, error) {
	env := append([]string{}, os.Environ()...)
	env = append(env,
		fmt.Sprintf("GOOS=%s", target.OS.GOOS()),
		fmt.Sprintf("GOARCH=%s", target.Arch.GOARCH()),
	)

	if target.OS != platform.Android {
		env = append(env, "CGO_ENABLED=0")
		return env, nil
	}

	ndkPath, err := GetNDKPath(ndkDir)
	if err != nil {
		return nil, err
	}

	cc, err := GetAndroidCompilerPath(
		ndkPath,
		target.Arch.String(),
		21,
	)
	if err != nil {
		return nil, err
	}

	env = append(env,
		"CGO_ENABLED=1",
		fmt.Sprintf("CC=%s", cc),
	)

	return env, nil
}

func moveArtifact(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyFileRaw(src, dst); err != nil {
		return fmt.Errorf("move artifact: %w", err)
	}

	_ = os.Remove(src)
	return nil
}

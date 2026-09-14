package compiler

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bgscan-builder/internal/platform"
)

// MinGoVersion defines the minimum toolchain version required to execute builds.
const MinGoVersion = "1.27.0"

// compiler implements the Compiler interface.
type compiler struct{}

// Build compiles bgscan for the requested target platform and stages the
// resulting binaries and configurations into the destination directory.
func (c *compiler) Build(target platform.Info, dest, projectDir, ndkDir, version string) error {
	goVersion, err := checkGoVersion()
	if err != nil {
		return err
	}

	if !isGoVersionSupported(goVersion, MinGoVersion) {
		return fmt.Errorf("go %s or newer is required", MinGoVersion)
	}

	workDir := projectDir
	if workDir == "" {
		var err error
		workDir, err = os.MkdirTemp("", "bgscan-*")
		if err != nil {
			return fmt.Errorf("create temporary workspace: %w", err)
		}
		defer func() { _ = os.RemoveAll(workDir) }()

		if err := CloneProject(workDir); err != nil {
			return err
		}
	} else if err := checkGoMod(workDir, "bgscan"); err != nil {
		return err
	}

	if err := c.prepareProjectFiles(workDir, dest); err != nil {
		return fmt.Errorf("prepare project files: %w", err)
	}

	if err := c.copyAssets(workDir, dest); err != nil {
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

	tidyOut := new(bytes.Buffer)
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = workDir
	tidyCmd.Env = env
	tidyCmd.Stdout = tidyOut
	tidyCmd.Stderr = tidyOut
	if err := tidyCmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\n%s", err, trimOutput(tidyOut))
	}

	buildArgs := []string{"build", "-o", outputName}
	if version != "" {
		buildArgs = append(buildArgs, "-ldflags", fmt.Sprintf("-s -w -X main.Version=%s", version))
	}
	buildArgs = append(buildArgs, "./cmd/bgscan")
	buildOut := new(bytes.Buffer)
	buildCmd := exec.Command("go", buildArgs...)
	buildCmd.Dir = workDir
	buildCmd.Env = env
	buildCmd.Stdout = buildOut
	buildCmd.Stderr = buildOut
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("go build failed: %w\n%s", err, trimOutput(buildOut))
	}

	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	srcBinary := filepath.Join(workDir, outputName)
	dstBinary := filepath.Join(dest, outputName)

	if err := moveArtifact(srcBinary, dstBinary); err != nil {
		return err
	}

	return nil
}

// trimOutput condenses captured subprocess output for inclusion in errors.
func trimOutput(buf *bytes.Buffer) string {
	if buf == nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
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

	if err := copyFile(src, dst); err != nil {
		return fmt.Errorf("move artifact: %w", err)
	}

	_ = os.Remove(src)
	return nil
}

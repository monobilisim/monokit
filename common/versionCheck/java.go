package common

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/shirou/gopsutil/v4/process"
)

type javaBinary struct {
	Path        string
	DisplayName string
	StoreKey    string
}

// javaVersionRegex matches the version line printed by `java -version` across
// OpenJDK, Eclipse Temurin, Oracle JDK, Corretto, Zulu, and GraalVM, e.g.:
//
//	openjdk version "25.0.1" 2025-10-21 LTS
//	java version "1.8.0_432"
var javaVersionRegex = regexp.MustCompile(`(?im)^(?:openjdk|java)\s+(?:full\s+)?version\s+"([^"]+)"`)

var javaGlobPatterns = []string{
	"/usr/lib/jvm/*/bin/java",
	"/usr/lib/jvm/*/jre/bin/java",
	"/usr/java/*/bin/java",
	"/usr/java/*/jre/bin/java",
	"/opt/*/bin/java",
	"/opt/*/jdk/bin/java",
	"/opt/*/jre/bin/java",
	"/opt/java/*/bin/java",
	"/usr/share/*/jdk/bin/java",
	"/usr/share/*/jre/bin/java",
}

// javaStoreKey converts a canonical Java binary path into a filesystem-safe
// healthdb key without path separators.
func javaStoreKey(path string) string {
	var b strings.Builder
	b.Grow(len(path) + 5)
	b.WriteString("java-")

	prevDash := true
	for _, r := range strings.ToLower(filepath.Clean(path)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}

	return strings.TrimRight(b.String(), "-")
}

// resolveExecutableJava resolves symlinks and verifies that path points to a
// regular executable file.
func resolveExecutableJava(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		resolved = filepath.Clean(path)
	} else {
		resolved = filepath.Clean(resolved)
	}

	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", false
	}

	return resolved, true
}

// discoverRunningJavaBinaries inspects running processes for java executables
// so bundled or custom-path JVMs (e.g. /usr/share/opensearch/jdk/bin/java) are
// found even when java is not on PATH.
func discoverRunningJavaBinaries() []string {
	procs, err := process.Processes()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to list running processes for Java discovery")
		return nil
	}

	var candidates []string
	for _, proc := range procs {
		if exe, err := proc.Exe(); err == nil && exe != "" && !strings.HasSuffix(exe, " (deleted)") {
			if filepath.Base(exe) == "java" {
				candidates = append(candidates, exe)
			}
		}

		if args, err := proc.CmdlineSlice(); err == nil && len(args) > 0 {
			if filepath.IsAbs(args[0]) && filepath.Base(args[0]) == "java" {
				candidates = append(candidates, args[0])
			}
		}
	}

	return candidates
}

// discoverJavaBinaries collects all unique Java binaries on the host from PATH,
// JAVA_HOME, running processes, and standard/bundled JVM directories.
func discoverJavaBinaries() []javaBinary {
	seen := make(map[string]bool)
	var binaries []javaBinary

	if pathBin, err := exec.LookPath("java"); err == nil {
		if resolved, ok := resolveExecutableJava(pathBin); ok {
			seen[resolved] = true
			binaries = append(binaries, javaBinary{
				Path:        resolved,
				DisplayName: "Java",
				StoreKey:    "java",
			})
		}
	}

	var candidates []string
	if javaHome := strings.TrimSpace(os.Getenv("JAVA_HOME")); javaHome != "" {
		candidates = append(candidates, filepath.Join(javaHome, "bin", "java"))
	}
	candidates = append(candidates, discoverRunningJavaBinaries()...)
	for _, pattern := range javaGlobPatterns {
		if matches, err := filepath.Glob(pattern); err == nil {
			candidates = append(candidates, matches...)
		}
	}

	var extraPaths []string
	for _, candidate := range candidates {
		resolved, ok := resolveExecutableJava(candidate)
		if !ok || seen[resolved] {
			continue
		}
		seen[resolved] = true
		extraPaths = append(extraPaths, resolved)
	}

	sort.Strings(extraPaths)
	for _, p := range extraPaths {
		binaries = append(binaries, javaBinary{
			Path:        p,
			DisplayName: fmt.Sprintf("Java (%s)", p),
			StoreKey:    javaStoreKey(p),
		})
	}

	return binaries
}

func parseJavaVersion(output string) string {
	m := javaVersionRegex.FindStringSubmatch(strings.TrimSpace(output))
	if len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func detectJavaVersion(binaryPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, binaryPath, "-version").CombinedOutput()
	version := parseJavaVersion(string(out))
	if version != "" {
		return version, nil
	}
	if err != nil {
		return "", err
	}
	return "", fmt.Errorf("could not parse java -version output")
}

// JavaCheck detects all installed or running Java binaries and records updates.
func JavaCheck() {
	binaries := discoverJavaBinaries()
	if len(binaries) == 0 {
		log.Debug().Msg("Java version not detected; skipping")
		addToNotInstalled("Java")
		return
	}

	for _, b := range binaries {
		version, err := detectJavaVersion(b.Path)
		if err != nil {
			log.Debug().Err(err).Str("binary", b.Path).Msg("Java version command failed")
			addToVersionErrors(fmt.Errorf("Error detecting %s version: %w", b.DisplayName, err))
			continue
		}
		recordVersion(b.DisplayName, b.StoreKey, version)
	}
}

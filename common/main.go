package common

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
	"math"

	"github.com/sirupsen/logrus"
)

var Config Common
var TmpPath string
var MonokitVersion = "devel"
var IgnoreLockfile bool // Flag to ignore lockfile check

func SplitSection(section string) {
	fmt.Println("\n" + section)
	fmt.Println("--------------------------------------------------")
}

func ContainsUint32(a uint32, b []uint32) bool {
	for _, c := range b {
		if a == c {
			return true
		}
	}
	return false
}

func IsEmptyOrWhitespace(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return false // Error opening file, consider it not empty
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) > 0 && !isWhitespace(text) {
			return false // Non-whitespace content found
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
		return false // Error reading file, consider it not empty
	}

	return true // No non-whitespace content found
}

func isWhitespace(text string) bool {
	for _, char := range text {
		if !unicode.IsSpace(char) {
			return false
		}
	}
	return true
}

func ConvertBytes(bytes uint64) string {
	var sizes = []string{"B", "KB", "MB", "GB", "TB", "EB"}

	if bytes == 0 {
		return "0 B"
	}

	// Convert to float64 to preserve decimal precision
	floatBytes := float64(bytes)
	var i int

	for i = 0; floatBytes >= 1024 && i < len(sizes)-1; i++ {
		floatBytes /= 1024
	}

	// Format with 2 decimal places for units >= MB
	if i >= 2 {
		return fmt.Sprintf("%.2f %s", floatBytes, sizes[i])
	}

	// For smaller units, use integer format
	if floatBytes > float64(math.MaxInt) {
		return fmt.Sprintf("%d %s", math.MaxInt, sizes[i])
	} else if floatBytes < float64(math.MinInt) {
		return fmt.Sprintf("%d %s", math.MinInt, sizes[i])
	}
	return fmt.Sprintf("%d %s", int(floatBytes), sizes[i])
}

func RemoveLockfile() {
	os.Remove(TmpDir + "/monokit.lock")
}

func Init() {
	var userMode bool = false

	// Check if user is root
	if os.Geteuid() != 0 {
		userMode = true
	}

	// Create TmpDir if it doesn't exist
	if _, err := os.Stat(TmpDir); os.IsNotExist(err) {
		err = os.MkdirAll(TmpDir, 0755)

		if err != nil {
			fmt.Println("Error creating tmp directory: \n" + TmpDir + "\n" + err.Error())
			os.Exit(1)
		}

	}

	if FileExists(TmpDir + "/monokit.lock") {
		// Check if a process is also running
		if !ProcGrep("monokit", true) {
			// Remove lockfile
			os.Remove(TmpDir + "/monokit.lock")
			LogDebug("Removed stale lockfile: " + TmpDir + "/monokit.lock")
		} else {
			if !IgnoreLockfile { // Check the flag before exiting
				fmt.Println("Monokit is already running (lockfile exists), exiting...")
				os.Exit(1)
			} else {
				LogDebug("Ignoring existing lockfile due to --ignore-lockfile flag.")
			}
		}
	}

	// Create lockfile only if not ignoring
	if !IgnoreLockfile {
		file, err := os.Create(TmpDir + "/monokit.lock")
		if err != nil {
			LogError("Failed to create lockfile: " + err.Error())
			// Decide if we should exit here or just warn
		} else {
			file.Close() // Close the file immediately after creation
			LogDebug("Created lockfile: " + TmpDir + "/monokit.lock")
		}
	}

	lvl, ok := os.LookupEnv("MONOKIT_LOGLEVEL")
	if !ok {
		os.Setenv("MONOKIT_LOGLEVEL", "info")
	}

	ll, err := logrus.ParseLevel(lvl)
	if err != nil {
		ll = logrus.InfoLevel
	}

	logrus.SetLevel(ll)

	// Check if env variable MONOKIT_NOCOLOR is set to true
	if os.Getenv("MONOKIT_NOCOLOR") == "true" || os.Getenv("MONOKIT_NOCOLOR") == "1" {
		RemoveColors()
	}

	LogInit(userMode)
	ConfInit("global", &Config)
}

func WriteToFile(filename string, data string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(data)
	return err
}

func IsInArray(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func FileExists(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return false
	}
	return true
}

func RemoveMonokitCrontab() {
	output, err := exec.Command("crontab", "-l").Output()
	if err == nil {
		var newCrontab []string
		var skipNext bool
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if skipNext {
				skipNext = false
				continue
			}
			// If this is an Ansible comment and next line contains monokit, skip both
			if strings.HasPrefix(line, "#Ansible:") && scanner.Scan() {
				nextLine := scanner.Text()
				if !strings.Contains(nextLine, "/usr/local/bin/monokit") {
					newCrontab = append(newCrontab, line, nextLine)
				}
			} else if !strings.Contains(line, "/usr/local/bin/monokit") {
				newCrontab = append(newCrontab, line)
			}
		}

		// Write the filtered crontab back
		cmd := exec.Command("crontab", "-")
		cmd.Stdin = strings.NewReader(strings.Join(newCrontab, "\n") + "\n")
		cmd.Run()
	}
}

func RemoveMonokit() {
	paths := []string{
		"/etc/mono.sh",
		"/tmp/mono",
		"/tmp/mono.sh",
		"/etc/mono",
		"/usr/local/bin/mono.sh",
		"/usr/local/bin/monokit",
	}

	// Remove specific files and directories
	for _, path := range paths {
		os.RemoveAll(path) // RemoveAll handles both files and directories
	}

	// Remove config files matching pattern
	matches, err := filepath.Glob("/etc/mono*.conf")
	if err == nil {
		for _, match := range matches {
			os.Remove(match)
		}
	}

	RemoveMonokitCrontab()

	fmt.Println("Monokit has been removed from the system")
}

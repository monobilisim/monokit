package common

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "math"
    "os"
    "os/exec"
    "path/filepath"
    "strconv"
    "strings"
    "time"
    "unicode"

    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
)

var Config Common
var TmpPath string
var MonokitVersion = "devel"
var IgnoreLockfile bool       // Flag to ignore lockfile check
var CleanupPluginsOnExit bool // Flag to clean up plugin processes before exit

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

	// Clamp bytes to math.MaxInt64 before converting to float64 to avoid incorrect conversion
	if bytes > uint64(math.MaxInt64) {
		bytes = uint64(math.MaxInt64)
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
	if floatBytes > float64(math.MaxInt64) {
		return fmt.Sprintf("%d %s", int64(math.MaxInt64), sizes[i])
	} else if floatBytes < 0 {
		return fmt.Sprintf("%d %s", int64(0), sizes[i])
	}
	// Clamp floatBytes to [0, math.MaxInt64] before converting to int64
	clamped := floatBytes
	if clamped > float64(math.MaxInt64) {
		clamped = float64(math.MaxInt64)
	} else if clamped < 0 {
		clamped = 0
	}
	// Ensure clamped is within int64 bounds before conversion
	if clamped < float64(math.MinInt64) {
		clamped = float64(math.MinInt64)
	} else if clamped > float64(math.MaxInt64) {
		clamped = float64(math.MaxInt64)
	}
	return fmt.Sprintf("%d %s", int64(clamped), sizes[i])
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
			log.Debug().
				Str("component", "lockfile").
				Str("action", "cleanup").
				Str("file", TmpDir+"/monokit.lock").
				Msg("Removed stale lockfile")
		} else {
			if !IgnoreLockfile { // Check the flag before exiting
				fmt.Println("Monokit is already running (lockfile exists), exiting...")
				os.Exit(1)
			} else {
				log.Debug().
					Str("component", "lockfile").
					Str("action", "ignore").
					Bool("ignore_flag", true).
					Msg("Ignoring existing lockfile due to --ignore-lockfile flag")
			}
		}
	}

	// Create lockfile only if not ignoring
	if !IgnoreLockfile {
		file, err := os.Create(TmpDir + "/monokit.lock")
		if err != nil {
			log.Error().
				Err(err).
				Str("component", "lockfile").
				Str("action", "create").
				Str("file", TmpDir+"/monokit.lock").
				Msg("Failed to create lockfile")
			// Decide if we should exit here or just warn
		} else {
			file.Close() // Close the file immediately after creation
			log.Debug().
				Str("component", "lockfile").
				Str("action", "create").
				Str("file", TmpDir+"/monokit.lock").
				Msg("Created lockfile successfully")
		}
	}

	// Check if env variable MONOKIT_NOCOLOR is set to true
	if os.Getenv("MONOKIT_NOCOLOR") == "true" || os.Getenv("MONOKIT_NOCOLOR") == "1" {
		RemoveColors()
	}

	ConfInit("global", &Config)

	log.Debug().
		Str("component", "init").
		Bool("user_mode", userMode).
		Str("tmp_dir", TmpDir).
		Bool("ignore_lockfile", IgnoreLockfile).
		Str("identifier", Config.Identifier).
		Msg("Monokit initialization completed")
}

// InitZerolog configures zerolog with enhanced structured logging
func InitZerolog() {
	// We also check userMode here because InitZerolog is the first thing that is called
	var userMode bool = false

	// Check if user is root
	if os.Geteuid() != 0 {
		userMode = true
	}

	// Set log level from environment variable
	lvl := os.Getenv("MONOKIT_LOGLEVEL")
	if lvl == "" {
		lvl = "info"
		os.Setenv("MONOKIT_LOGLEVEL", "info")
	}

	// Parse log level
	level, err := zerolog.ParseLevel(lvl)
	if err != nil {
		level = zerolog.InfoLevel // Default to info instead of debug for production safety
		log.Warn().
			Str("provided_level", lvl).
			Str("default_level", level.String()).
			Msg("Invalid log level provided, using default")
	}
	zerolog.SetGlobalLevel(level)

	// Configure enhanced caller marshaling with source info
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Base(file) + ":" + fmt.Sprintf("%d", line)
	}

	// Configure time formatting for better readability
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFieldName = "timestamp"

	// Configure field names for consistency
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"
	zerolog.ErrorFieldName = "error"

	// Determine log file path
	logfilePath := "/var/log/monokit.log"
	if userMode {
		xdgStateHome := os.Getenv("XDG_STATE_HOME")
		if xdgStateHome == "" {
			xdgStateHome = os.Getenv("HOME") + "/.local/state"
		}

		// Create the directory if it doesn't exist
		if _, err := os.Stat(xdgStateHome + "/monokit"); os.IsNotExist(err) {
			os.MkdirAll(xdgStateHome+"/monokit", 0755)
		}

		logfilePath = xdgStateHome + "/monokit/monokit.log"
	}

	// Open log file with proper error handling
	logFile, err := os.OpenFile(logfilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stderr if we can't open the log file
		fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v, falling back to stderr\n", logfilePath, err)
		logFile = os.Stderr
	}

	// Create console writer for pretty stdout output
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
		NoColor:    os.Getenv("MONOKIT_NOCOLOR") == "true" || os.Getenv("MONOKIT_NOCOLOR") == "1",
		FieldsExclude: []string{
			"component", // Reduce noise in console output
		},
	}

	// Use MultiLevelWriter to send JSON to file and pretty to stdout
	output := zerolog.MultiLevelWriter(consoleWriter, logFile)

	// Create base logger with rich context
	logger := zerolog.New(output).
		Level(level).
		With().
		Timestamp().
		Caller().
		Str("component", "monokit").
		Str("version", MonokitVersion).
		Str("pid", fmt.Sprintf("%d", os.Getpid())).
		Bool("user_mode", userMode)

	// Add runtime information
	if hostname, err := os.Hostname(); err == nil {
		logger = logger.Str("hostname", hostname)
	}

	// Add environment context
	if env := os.Getenv("MONOKIT_ENV"); env != "" {
		logger = logger.Str("environment", env)
	}

	// Finalize the context
	finalLogger := logger.Logger()

	// Add API hook if client configuration exists
	apiHook := NewZerologAPIHook(ScriptName)
	if apiHook.GetSubmitter().enabled {
		finalLogger = finalLogger.Hook(apiHook)
		finalLogger.Debug().
			Str("component", "logging").
			Str("hook", "api_submission").
			Bool("enabled", true).
			Msg("API log submission hook enabled")
	} else {
		finalLogger.Debug().
			Str("component", "logging").
			Str("hook", "api_submission").
			Bool("enabled", false).
			Msg("API log submission hook disabled")
	}

    // Set the global logger
	log.Logger = finalLogger

    // Start background log retention enforcer (default: 20 days)
    retentionDays := 20
    if v := os.Getenv("MONOKIT_LOG_RETENTION_DAYS"); v != "" {
        if n, err := strconv.Atoi(v); err == nil && n > 0 {
            retentionDays = n
        }
    }
    // Prune once immediately, then schedule periodic pruning in background
    _ = pruneLogFileByAge(logfilePath, time.Duration(retentionDays)*24*time.Hour)
    go startLogRetentionEnforcer(logfilePath, retentionDays)

    // Log successful initialization with comprehensive context
	log.Debug().
		Str("component", "logging").
		Str("level", level.String()).
		Str("log_file", logfilePath).
		Bool("user_mode", userMode).
		Bool("colors_enabled", !(os.Getenv("MONOKIT_NOCOLOR") == "true" || os.Getenv("MONOKIT_NOCOLOR") == "1")).
		Bool("api_hook_enabled", apiHook.GetSubmitter().enabled).
		Msg("Enhanced zerolog initialized successfully")
}

// startLogRetentionEnforcer launches a goroutine that enforces log retention
// by keeping only log lines newer than the configured retention window.
// It performs an initial prune shortly after startup and then repeats daily.
func startLogRetentionEnforcer(logfilePath string, retentionDays int) {
    // Schedule daily
    ticker := time.NewTicker(24 * time.Hour)
    for range ticker.C {
        _ = pruneLogFileByAge(logfilePath, time.Duration(retentionDays)*24*time.Hour)
    }
}

// pruneLogFileByAge trims the given JSON-lines log file in-place so it only
// contains entries whose timestamp is within the given maxAge from now.
// It is designed to work with writers that keep the file descriptor open by
// using a copy-truncate approach to avoid inode replacement.
func pruneLogFileByAge(path string, maxAge time.Duration) error {
    // Fast path: ensure file exists and is regular
    info, err := os.Stat(path)
    if err != nil {
        return err
    }
    if !info.Mode().IsRegular() {
        return fmt.Errorf("not a regular file: %s", path)
    }

    // Open for reading
    src, err := os.Open(path)
    if err != nil {
        return err
    }
    defer src.Close()

    // Create a temporary file alongside the original
    dir := filepath.Dir(path)
    tmp, err := os.CreateTemp(dir, "monokit-log-prune-*.tmp")
    if err != nil {
        return err
    }
    tmpPath := tmp.Name()

    // Ensure cleanup of tmp on exit
    defer func() {
        tmp.Close()
        os.Remove(tmpPath)
    }()

    cutoff := time.Now().Add(-maxAge)

    // Scan line by line; increase buffer in case of long lines
    scanner := bufio.NewScanner(src)
    const maxScanTokenSize = 1024 * 1024 // 1 MiB per line
    buf := make([]byte, 0, 64*1024)
    scanner.Buffer(buf, maxScanTokenSize)

    writer := bufio.NewWriter(tmp)

    kept := 0
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" {
            continue
        }
        // Parse timestamp from JSON; on error, keep the line (be conservative)
        ts, perr := extractTimestampRFC3339Nano(line)
        if perr != nil || ts.IsZero() || !ts.Before(cutoff) {
            // Keep this line
            if _, err := writer.WriteString(line + "\n"); err != nil {
                return err
            }
            kept++
        }
    }
    if err := scanner.Err(); err != nil {
        // If we failed scanning, do not modify the log file
        return err
    }
    if err := writer.Flush(); err != nil {
        return err
    }

    // Copy-truncate: replace contents of original file without changing inode
    // Open the original for read/write
    dst, err := os.OpenFile(path, os.O_WRONLY, 0)
    if err != nil {
        return err
    }
    defer dst.Close()

    if err := dst.Truncate(0); err != nil {
        return err
    }
    if _, err := dst.Seek(0, 0); err != nil {
        return err
    }

    // Rewind tmp and copy back
    if _, err := tmp.Seek(0, 0); err != nil {
        return err
    }
    if _, err := io.Copy(dst, tmp); err != nil {
        return err
    }

    // Log a brief message to the logger; avoid recursion by using stderr on failure only
    log.Debug().
        Str("component", "logging").
        Str("action", "prune").
        Str("file", path).
        Int("kept_lines", kept).
        Dur("max_age", maxAge).
        Msg("Pruned log file by age")

    return nil
}

// extractTimestampRFC3339Nano extracts the timestamp field from a JSON line
// and parses it as RFC3339 with nanoseconds. Returns zero time on failure.
func extractTimestampRFC3339Nano(line string) (time.Time, error) {
    // Fast path: find the "timestamp":"..." substring without full JSON decode
    // but fall back to json decode if not found.
    const key = "\"timestamp\""
    idx := strings.Index(line, key)
    if idx >= 0 {
        // Find the next colon and quote
        colon := strings.Index(line[idx+len(key):], ":")
        if colon >= 0 {
            // Skip colon and optional spaces
            j := idx + len(key) + colon + 1
            for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
                j++
            }
            if j < len(line) && line[j] == '"' {
                j++
                k := j
                for k < len(line) {
                    if line[k] == '"' && line[k-1] != '\\' {
                        break
                    }
                    k++
                }
                if k <= len(line) {
                    tsStr := line[j:k]
                    if t, err := time.Parse(time.RFC3339Nano, tsStr); err == nil {
                        return t, nil
                    }
                    if t, err := time.Parse(time.RFC3339, tsStr); err == nil {
                        return t, nil
                    }
                }
            }
        }
    }
    // Fallback: decode minimal JSON
    var tmp map[string]interface{}
    if err := json.Unmarshal([]byte(line), &tmp); err != nil {
        return time.Time{}, err
    }
    v, _ := tmp["timestamp"].(string)
    if v == "" {
        return time.Time{}, fmt.Errorf("no timestamp")
    }
    if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
        return t, nil
    }
    if t, err := time.Parse(time.RFC3339, v); err == nil {
        return t, nil
    }
    return time.Time{}, fmt.Errorf("invalid timestamp")
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

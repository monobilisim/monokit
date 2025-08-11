package common

import (
    "bufio"
    "fmt"
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
    lumberjack "gopkg.in/natefinch/lumberjack.v2"
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

    // Ensure we can create/write the log file (permission check)
    var fileUsable bool = true
    if f, err := os.OpenFile(logfilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err != nil {
        fmt.Fprintf(os.Stderr, "Failed to open log file %s: %v, using stdout only\n", logfilePath, err)
        fileUsable = false
    } else {
        _ = f.Close()
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

    // Configure size- and age-based rotation with lumberjack
    maxSizeMB := getEnvInt("MONOKIT_LOG_MAX_SIZE_MB", 100)        // default 100 MB
    maxBackups := getEnvInt("MONOKIT_LOG_MAX_BACKUPS", 7)         // default 7 files
    retentionDays := getEnvInt("MONOKIT_LOG_RETENTION_DAYS", 20) // default 20 days
    compress := getEnvBool("MONOKIT_LOG_COMPRESS", true)

    var output zerolog.LevelWriter
    if fileUsable {
        lj := &lumberjack.Logger{
            Filename:   logfilePath,
            MaxSize:    maxSizeMB,
            MaxBackups: maxBackups,
            MaxAge:     retentionDays,
            Compress:   compress,
        }
        // Write pretty to stdout and JSON to rotating file
        output = zerolog.MultiLevelWriter(consoleWriter, lj)
    } else {
        // Fallback to stdout only
        output = zerolog.MultiLevelWriter(consoleWriter)
    }

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

    // Log successful initialization with comprehensive context
	log.Debug().
		Str("component", "logging").
		Str("level", level.String()).
		Str("log_file", logfilePath).
        Int("max_size_mb", maxSizeMB).
        Int("max_backups", maxBackups).
        Int("retention_days", retentionDays).
        Bool("compress", compress).
		Bool("user_mode", userMode).
		Bool("colors_enabled", !(os.Getenv("MONOKIT_NOCOLOR") == "true" || os.Getenv("MONOKIT_NOCOLOR") == "1")).
		Bool("api_hook_enabled", apiHook.GetSubmitter().enabled).
		Msg("Enhanced zerolog initialized successfully")
}

// getEnvInt reads an integer environment variable with a default.
func getEnvInt(name string, def int) int {
    v := strings.TrimSpace(os.Getenv(name))
    if v == "" {
        return def
    }
    if n, err := strconv.Atoi(v); err == nil {
        return n
    }
    return def
}

// getEnvBool reads a boolean environment variable with a default.
// Accepts: 1/0, true/false, yes/no, on/off (case-insensitive)
func getEnvBool(name string, def bool) bool {
    v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
    if v == "" {
        return def
    }
    switch v {
    case "1", "true", "yes", "on":
        return true
    case "0", "false", "no", "off":
        return false
    default:
        return def
    }
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

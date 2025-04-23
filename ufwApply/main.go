package ufwApply

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/spf13/cobra"
)

// UfwCmd represents the root command for UFW operations
var UfwCmd = &cobra.Command{
	Use:   "ufw",
	Short: "UFW operations",
	Long:  `UFW operations for managing firewall rules`,
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		noCmdOut, _ := cmd.Flags().GetBool("no-cmdout")

		if err := Execute(dryRun, noCmdOut); err != nil {
			common.LogError(fmt.Sprintf("UFW execution failed: %v", err))
			os.Exit(1)
		}
	},
}

func init() {
	UfwCmd.Flags().BoolP("dry-run", "d", false, "Print commands without executing them")
	UfwCmd.Flags().BoolP("no-cmdout", "n", false, "Suppress command output")
}

// Config represents the structure for ufw configuration
type Config struct {
	RuleURLs    []string `mapstructure:"rule_urls"`
	StaticRules []string `mapstructure:"static_rules"`
	RulesetDir  string   `mapstructure:"ruleset_dir"`
	RulesDir    string   `mapstructure:"rules_dir"`
	TmpDirBase  string   `mapstructure:"tmp_dir_base"` // Base for temp dir, timestamp will be added
}

var (
	dryRun   bool
	noCmdOut bool

	// Default paths, can be overridden by config
	defaultMonoDir    = "/etc/mono"
	defaultRulesetDir = "ufw-applier-ruleset" // Relative to monoDir
	defaultRulesDir   = "ufw-applier"         // Relative to monoDir
	defaultTmpDirBase = "ufw-applier"         // Relative to os.TempDir()
)

// Execute runs the ufw applier with the given flags
func Execute(dryRunFlag bool, noCmdOutFlag bool) error {
	dryRun = dryRunFlag
	noCmdOut = noCmdOutFlag

	config, tmpDir, err := loadUfwConfig()
	if err != nil {
		return fmt.Errorf("error loading UFW configuration: %v", err)
	}
	defer os.RemoveAll(tmpDir) // Ensure temp dir cleanup

	// Create required directories
	if err := createDirectories(config, tmpDir); err != nil {
		return fmt.Errorf("error creating required directories: %v", err)
	}

	// Process rule URLs
	processedFiles := make(map[string]bool)
	_ = processRuleURLs(config, tmpDir, processedFiles)

	// Clean up unused rules derived from URLs
	cleanupUnusedRules(config, processedFiles)

	// Process static rules (check, remove old, apply new)
	checkAndRemoveStatic(config)
	applyStatic(config)

	common.LogInfo("UFW rule application process completed.")
	return nil
}

func loadUfwConfig() (*Config, string, error) {
	var ufwConfig Config

	// Use common.ConfInit to load the configuration
	common.ConfInit("ufw", &ufwConfig) // This handles reading, initialization, and unmarshalling

	// Log loaded URLs for debugging (Viper instance is managed within ConfInit)
	// We need a way to access the viper instance used by ConfInit or re-read values if needed.
	// For now, let's assume ConfInit succeeded and ufwConfig is populated.
	// Re-reading with a local viper instance just for logging seems redundant.
	// Let's rely on the unmarshalled struct for counts.

	common.LogDebug(fmt.Sprintf("Loaded config via common.ConfInit: RuleURLs count=%d, StaticRules count=%d",
		len(ufwConfig.RuleURLs), len(ufwConfig.StaticRules)))

	// If detailed URL logging is still needed, ConfInit might need adjustment or
	// we might need to access the global viper instance if ConfInit uses it.
	// Assuming basic count logging is sufficient for now.

	monoDir := defaultMonoDir // Keep default monoDir definition

	common.LogDebug(fmt.Sprintf("Loaded config: RuleURLs count=%d, StaticRules count=%d",
		len(ufwConfig.RuleURLs), len(ufwConfig.StaticRules)))

	// Apply defaults if paths are not set in config
	if ufwConfig.RulesetDir == "" {
		ufwConfig.RulesetDir = filepath.Join(monoDir, defaultRulesetDir)
	}
	if ufwConfig.RulesDir == "" {
		ufwConfig.RulesDir = filepath.Join(monoDir, defaultRulesDir)
	}
	if ufwConfig.TmpDirBase == "" {
		ufwConfig.TmpDirBase = filepath.Join(os.TempDir(), defaultTmpDirBase)
	}

	// Create a unique temp directory for this run
	tmpDir := fmt.Sprintf("%s-%s", ufwConfig.TmpDirBase, time.Now().Format("20060102150405"))

	return &ufwConfig, tmpDir, nil
}

func createDirectories(config *Config, tmpDir string) error {
	dirs := []string{tmpDir, config.RulesDir, config.RulesetDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			common.LogWarn(fmt.Sprintf("Failed to create directory %s: %v", dir, err))
		} else {
			common.LogDebug(fmt.Sprintf("Ensured directory exists: %s", dir))
		}
	}
	return nil
}

func processRuleURLs(config *Config, tmpDir string, processedFiles map[string]bool) int {
	processed := 0
	urlToFile := make(map[string]string) // Map to track URL to filename mappings
	common.LogInfo(fmt.Sprintf("Processing %d rule URLs...", len(config.RuleURLs)))

	// First pass: check for duplicate filenames
	for _, rule := range config.RuleURLs {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			continue
		}
		currentURL := parts[0]
		filename := filepath.Base(currentURL)
		if existingURL, exists := urlToFile[filename]; exists {
			common.LogWarn(fmt.Sprintf("Duplicate filename detected: %s (URLs: %s and %s)", filename, existingURL, currentURL))
		} else {
			urlToFile[filename] = currentURL
		}
	}

	for _, rule := range config.RuleURLs {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			common.LogWarn(fmt.Sprintf("Invalid rule URL format, skipping: %s", rule))
			continue
		}

		ruleURL := parts[0]
		ruleProtocol := parts[1]
		rulePort := parts[2]

		var ruleDescription string
		if len(parts) > 3 {
			ruleDescription = strings.Join(parts[3:], " ")
		} else {
			ruleDescription = "default"
		}

		ruleFile := filepath.Base(ruleURL)
		// Debug logging for the rule being processed
		common.LogDebug(fmt.Sprintf("Current rule: URL=%s -> File=%s", ruleURL, ruleFile))

		tmpFile := filepath.Join(tmpDir, ruleFile+"-tmp")
		destFile := filepath.Join(config.RulesDir, ruleFile)
		rulesetFile := filepath.Join(config.RulesetDir, ruleFile)

		common.LogDebug(fmt.Sprintf("Processing rule: URL=%s, Proto=%s, Port=%s, Desc=%s", ruleURL, ruleProtocol, rulePort, ruleDescription))

		if err := downloadFile(ruleURL, tmpFile); err != nil {
			common.LogError(fmt.Sprintf("Failed to download %s: %v", ruleURL, err))
			continue
		}
		common.LogDebug(fmt.Sprintf("Downloaded %s to %s", ruleURL, tmpFile))

		needsUpdate := true
		if common.FileExists(destFile) {
			origSum, errOrig := calculateSHA256(destFile)
			newSum, errNew := calculateSHA256(tmpFile)

			if errOrig != nil {
				common.LogWarn(fmt.Sprintf("Could not calculate SHA256 for existing file %s: %v. Forcing update.", destFile, errOrig))
			} else if errNew != nil {
				common.LogError(fmt.Sprintf("Could not calculate SHA256 for downloaded file %s: %v. Skipping update.", tmpFile, errNew))
				os.Remove(tmpFile)
				continue
			} else {
				var portFileContent string
				if common.FileExists(rulesetFile) {
					data, err := os.ReadFile(rulesetFile)
					if err != nil {
						common.LogWarn(fmt.Sprintf("Could not read ruleset file %s: %v. Forcing update.", rulesetFile, err))
					} else {
						portFileContent = string(data)
					}
				}

				portFileNew := fmt.Sprintf("%s %s", ruleProtocol, rulePort)
				if ruleDescription != "default" {
					portFileNew += " " + ruleDescription
				}

				if origSum == newSum && portFileContent == portFileNew {
					common.LogInfo(fmt.Sprintf("Rule file %s is up-to-date. No changes needed.", ruleFile))
					os.Remove(tmpFile)
					needsUpdate = false
				} else {
					if origSum != newSum {
						common.LogInfo(fmt.Sprintf("Content mismatch for %s (SHA: %s -> %s). Updating.", ruleFile, origSum, newSum))
					}
					if portFileContent != portFileNew {
						common.LogInfo(fmt.Sprintf("Metadata mismatch for %s ('%s' -> '%s'). Updating.", ruleFile, portFileContent, portFileNew))
					}
					if err := removeFileRules(config, destFile); err != nil {
						common.LogError(fmt.Sprintf("Error removing old rules for %s: %v. Skipping update.", destFile, err))
						os.Remove(tmpFile)
						continue
					}
					if err := os.Remove(destFile); err != nil && !os.IsNotExist(err) {
						common.LogWarn(fmt.Sprintf("Could not remove old rule file %s: %v", destFile, err))
					}
				}
			}
		} else {
			common.LogInfo(fmt.Sprintf("New rule file %s. Applying.", ruleFile))
		}

		if needsUpdate {
			if err := os.Rename(tmpFile, destFile); err != nil {
				common.LogError(fmt.Sprintf("Failed to move %s to %s: %v", tmpFile, destFile, err))
				continue
			}
			common.LogDebug(fmt.Sprintf("Moved %s to %s", tmpFile, destFile))

			if err := applyFileRules(config, destFile, ruleProtocol, rulePort, ruleDescription); err != nil {
				common.LogError(fmt.Sprintf("Error applying rules from %s: %v", destFile, err))
			}
		}

		processed++
		processedFiles[ruleFile] = true
		common.LogDebug(fmt.Sprintf("Marked %s as processed (from URL %s)", ruleFile, ruleURL))
	}

	common.LogInfo(fmt.Sprintf("Processed %d rule URLs", processed))
	return processed
}

func cleanupUnusedRules(config *Config, processedFiles map[string]bool) {
	common.LogInfo("Cleaning up unused URL-based rule files...")
	files, err := os.ReadDir(config.RulesDir)
	if err != nil {
		common.LogError(fmt.Sprintf("Error reading rules directory %s: %v", config.RulesDir, err))
		return
	}

	// Debug logging for processed files
	// Debug logging for processed files
	processedCount := len(processedFiles)
	processedNames := make([]string, 0, processedCount)
	for name := range processedFiles {
		processedNames = append(processedNames, name)
	}
	common.LogDebug(fmt.Sprintf("Tracking %d files from config: %v", processedCount, processedNames))

	// Debug logging for existing files
	existingNames := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && !strings.HasPrefix(file.Name(), "static-") {
			existingNames = append(existingNames, file.Name())
		}
	}
	common.LogDebug(fmt.Sprintf("Found %d files on disk: %v", len(existingNames), existingNames))

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), "static-") {
			continue
		}

		if !processedFiles[file.Name()] {
			filePath := filepath.Join(config.RulesDir, file.Name())
			common.LogInfo(fmt.Sprintf("Rule file %s is no longer listed in config URLs. Removing associated rules and file.", file.Name()))

			if err := removeFileRules(config, filePath); err != nil {
				common.LogError(fmt.Sprintf("Error removing rules for obsolete file %s: %v", filePath, err))
			}

			rulesetFile := filepath.Join(config.RulesetDir, file.Name())
			if err := os.Remove(rulesetFile); err != nil && !os.IsNotExist(err) {
				common.LogWarn(fmt.Sprintf("Could not remove ruleset file %s: %v", rulesetFile, err))
			} else if err == nil {
				common.LogDebug(fmt.Sprintf("Removed ruleset file %s", rulesetFile))
			}

			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				common.LogWarn(fmt.Sprintf("Could not remove rule file %s: %v", filePath, err))
			} else if err == nil {
				common.LogDebug(fmt.Sprintf("Removed rule file %s", filePath))
			}
		}
	}
	common.LogInfo("Finished cleaning up unused URL-based rules.")
}

func removeFileRules(config *Config, ruleContentFilePath string) error {
	ruleFileName := filepath.Base(ruleContentFilePath)
	rulesetFile := filepath.Join(config.RulesetDir, ruleFileName)

	if !common.FileExists(rulesetFile) {
		common.LogWarn(fmt.Sprintf("Cannot remove rules for %s: Ruleset file %s not found.", ruleFileName, rulesetFile))
		return nil
	}

	data, err := os.ReadFile(rulesetFile)
	if err != nil {
		return fmt.Errorf("error reading rule definition file %s: %w", rulesetFile, err)
	}

	parts := strings.Fields(string(data))
	if len(parts) < 2 {
		return fmt.Errorf("invalid rule format in definition file %s: %s", rulesetFile, string(data))
	}

	protocol := parts[0]
	port := parts[1]
	var description string
	if len(parts) > 2 {
		description = strings.Join(parts[2:], " ")
	} else {
		description = ruleFileName
	}

	common.LogInfo(fmt.Sprintf("Removing UFW rules defined in %s (Proto: %s, Port: %s, Desc: %s)", ruleContentFilePath, protocol, port, description))

	file, err := os.Open(ruleContentFilePath)
	if err != nil {
		return fmt.Errorf("error opening rule content file %s: %w", ruleContentFilePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var removalErrors []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ipParts := strings.Fields(line)
		if len(ipParts) == 0 {
			continue
		}
		ipAddress := ipParts[0]

		comment := description
		commands := buildUfwCommand("delete allow", ipAddress, protocol, port, comment)

		if !noCmdOut {
			for _, cmd := range commands {
				common.LogInfo(fmt.Sprintf("Executing: %s", cmd))
			}
		}

		if !dryRun {
			if err := execUfwCommand(commands); err != nil {
				errMsg := fmt.Sprintf("failed to execute delete command: %v", err)
				common.LogError(errMsg)
				removalErrors = append(removalErrors, errMsg)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning rule content file %s: %w", ruleContentFilePath, err)
	}

	if len(removalErrors) > 0 {
		return fmt.Errorf("encountered %d errors during rule removal for %s:\n%s", len(removalErrors), ruleFileName, strings.Join(removalErrors, "\n"))
	}

	common.LogInfo(fmt.Sprintf("Finished removing rules for %s", ruleFileName))
	return nil
}

func applyFileRules(config *Config, ruleContentFilePath string, protocol, port, description string) error {
	ruleFileName := filepath.Base(ruleContentFilePath)
	rulesetFile := filepath.Join(config.RulesetDir, ruleFileName)

	common.LogInfo(fmt.Sprintf("Applying UFW rules from %s (Proto: %s, Port: %s, Desc: %s)", ruleContentFilePath, protocol, port, description))

	ruleDefinitionContent := fmt.Sprintf("%s %s", protocol, port)
	if description != "default" && description != "" {
		ruleDefinitionContent += " " + description
	} else {
		description = ruleFileName
	}

	err := os.WriteFile(rulesetFile, []byte(ruleDefinitionContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing rule definition file %s: %w", rulesetFile, err)
	}
	common.LogDebug(fmt.Sprintf("Wrote rule definition to %s", rulesetFile))

	file, err := os.Open(ruleContentFilePath)
	if err != nil {
		return fmt.Errorf("error opening rule content file %s: %w", ruleContentFilePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var applyErrors []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		ipParts := strings.Fields(line)
		if len(ipParts) == 0 {
			continue
		}
		ipAddress := ipParts[0]

		comment := description
		commands := buildUfwCommand("allow", ipAddress, protocol, port, comment)

		if !noCmdOut {
			for _, cmd := range commands {
				common.LogInfo(fmt.Sprintf("Executing: %s", cmd))
			}
		}

		if !dryRun {
			if err := execUfwCommand(commands); err != nil {
				errMsg := fmt.Sprintf("failed to execute allow command: %v", err)
				common.LogError(errMsg)
				applyErrors = append(applyErrors, errMsg)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error scanning rule content file %s: %w", ruleContentFilePath, err)
	}

	if len(applyErrors) > 0 {
		return fmt.Errorf("encountered %d errors during rule application for %s:\n%s", len(applyErrors), ruleFileName, strings.Join(applyErrors, "\n"))
	}

	common.LogInfo(fmt.Sprintf("Finished applying rules from %s", ruleFileName))
	return nil
}

func removeStaticRule(ruleDefinition string) error {
	parts := strings.Fields(ruleDefinition)
	if len(parts) < 3 {
		return fmt.Errorf("invalid static rule format for removal: %s", ruleDefinition)
	}

	ip := parts[0]
	protocol := parts[1]
	port := parts[2]
	var description string
	if len(parts) > 3 {
		description = strings.Join(parts[3:], " ")
	} else {
		description = fmt.Sprintf("static-%s-%s-%s", ip, protocol, port)
	}

	common.LogInfo(fmt.Sprintf("Removing static rule: %s", ruleDefinition))

	commands := buildUfwCommand("delete allow", ip, protocol, port, description)

	if !noCmdOut {
		for _, cmd := range commands {
			common.LogInfo(fmt.Sprintf("Executing: %s", cmd))
		}
	}

	if !dryRun {
		if err := execUfwCommand(commands); err != nil {
			common.LogError(fmt.Sprintf("Failed to remove static rule '%s': %v", ruleDefinition, err))
			return err
		}
	}
	return nil
}

func applyStatic(config *Config) {
	common.LogInfo(fmt.Sprintf("Applying %d static rules...", len(config.StaticRules)))
	appliedCount := 0
	skippedCount := 0
	errorCount := 0

	for _, rule := range config.StaticRules {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			common.LogWarn(fmt.Sprintf("Invalid static rule format, skipping: %s", rule))
			errorCount++
			continue
		}

		ip := parts[0]
		protocol := parts[1]
		port := parts[2]
		var description string
		if len(parts) > 3 {
			description = strings.Join(parts[3:], " ")
		}

		markerFileName := fmt.Sprintf("static-%s-%s-%s", ip, protocol, port)
		markerFilePath := filepath.Join(config.RulesetDir, markerFileName)

		needsApply := true
		if common.FileExists(markerFilePath) {
			existingContent, err := os.ReadFile(markerFilePath)
			if err != nil {
				common.LogWarn(fmt.Sprintf("Could not read static rule marker %s: %v. Re-applying rule.", markerFilePath, err))
			} else if string(existingContent) == rule {
				needsApply = false
				skippedCount++
			} else {
				common.LogInfo(fmt.Sprintf("Static rule marker %s exists but content differs ('%s' vs '%s'). Removing old and re-applying.", markerFileName, string(existingContent), rule))
				if err := removeStaticRule(string(existingContent)); err != nil {
					common.LogError(fmt.Sprintf("Failed to remove outdated static rule '%s' before applying update: %v", string(existingContent), err))
					errorCount++
					continue
				}
			}
		}

		if needsApply {
			if description == "" {
				description = markerFileName
			}

			commands := buildUfwCommand("allow", ip, protocol, port, description)

			if !noCmdOut {
				for _, cmd := range commands {
					common.LogInfo(fmt.Sprintf("Executing: %s", cmd))
				}
			}

			if !dryRun {
				if err := execUfwCommand(commands); err != nil {
					common.LogError(fmt.Sprintf("Failed to apply static rule '%s': %v", rule, err))
					errorCount++
					continue
				}
			}

			err := os.WriteFile(markerFilePath, []byte(rule), 0644)
			if err != nil {
				common.LogError(fmt.Sprintf("Failed to write static rule marker file %s: %v", markerFilePath, err))
				errorCount++
			} else {
				common.LogDebug(fmt.Sprintf("Applied static rule and wrote marker: %s", markerFileName))
				appliedCount++
			}
		}
	}
	common.LogInfo(fmt.Sprintf("Finished applying static rules. Applied: %d, Skipped: %d, Errors: %d", appliedCount, skippedCount, errorCount))
}

func checkAndRemoveStatic(config *Config) {
	common.LogInfo("Checking for obsolete static rules...")
	removedCount := 0
	errorCount := 0

	configuredRules := make(map[string]string)
	for _, rule := range config.StaticRules {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			continue
		}
		ip, protocol, port := parts[0], parts[1], parts[2]
		markerFileName := fmt.Sprintf("static-%s-%s-%s", ip, protocol, port)
		configuredRules[markerFileName] = rule
	}
	common.LogDebug(fmt.Sprintf("Found %d configured static rules.", len(configuredRules)))

	dirEntries, err := os.ReadDir(config.RulesetDir)
	if err != nil {
		if os.IsNotExist(err) {
			common.LogWarn(fmt.Sprintf("Ruleset directory %s does not exist. Skipping static rule cleanup.", config.RulesetDir))
			return
		}
		common.LogError(fmt.Sprintf("Error reading ruleset directory %s: %v", config.RulesetDir, err))
		return
	}

	for _, entry := range dirEntries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "static-") {
			continue
		}

		markerFileName := entry.Name()
		markerFilePath := filepath.Join(config.RulesetDir, markerFileName)

		if currentRuleDef, found := configuredRules[markerFileName]; found {
			existingContent, err := os.ReadFile(markerFilePath)
			if err != nil {
				common.LogWarn(fmt.Sprintf("Could not read static rule marker %s: %v. Rule might be reapplied later.", markerFilePath, err))
				continue
			}
			if string(existingContent) != currentRuleDef {
				common.LogInfo(fmt.Sprintf("Static rule marker %s content ('%s') differs from config ('%s'). Removing old rule.", markerFileName, string(existingContent), currentRuleDef))
				if err := removeStaticRule(string(existingContent)); err != nil {
					common.LogError(fmt.Sprintf("Failed to remove outdated static rule from marker %s: %v", markerFileName, err))
					errorCount++
				}
			}
		} else {
			common.LogInfo(fmt.Sprintf("Static rule marker %s found, but rule is not in config. Removing.", markerFileName))

			ruleToRemove, err := os.ReadFile(markerFilePath)
			if err != nil {
				common.LogError(fmt.Sprintf("Could not read obsolete marker file %s to remove rule: %v", markerFilePath, err))
				errorCount++
				continue
			}

			if err := removeStaticRule(string(ruleToRemove)); err != nil {
				common.LogError(fmt.Sprintf("Failed to remove obsolete static rule '%s': %v", string(ruleToRemove), err))
				errorCount++
			}

			if err := os.Remove(markerFilePath); err != nil {
				common.LogError(fmt.Sprintf("Failed to remove obsolete static rule marker %s: %v", markerFilePath, err))
				errorCount++
			} else {
				common.LogDebug(fmt.Sprintf("Removed obsolete static rule marker: %s", markerFileName))
				removedCount++
			}
		}
	}
	common.LogInfo(fmt.Sprintf("Finished checking static rules. Removed: %d, Errors: %d", removedCount, errorCount))
}

func downloadFile(url, filePath string) error {
	common.LogDebug(fmt.Sprintf("Downloading %s to %s", url, filePath))
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("http get failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("bad status: %s. Body: %s", resp.Status, string(bodyBytes))
	}

	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", filePath, err)
	}
	return nil
}

func calculateSHA256(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return calculateSHA256FromBytes(data), nil
}

func calculateSHA256FromBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}

func buildUfwCommand(action, ip, protocol, port, comment string) []string {
	var commands []string

	if comment == "" {
		comment = "monokit-ufw-unspecified"
	}
	sanitizedComment := strings.ReplaceAll(comment, "'", "")
	sanitizedComment = strings.ReplaceAll(sanitizedComment, "`", "")
	sanitizedComment = strings.ReplaceAll(sanitizedComment, "$", "")

	buildCommand := func(proto string) string {
		var cmdParts []string
		cmdParts = append(cmdParts, "ufw", action)

		if proto != "all" && proto != "" {
			cmdParts = append(cmdParts, "proto", proto)
		}

		cmdParts = append(cmdParts, "from")
		if ip == "any" || ip == "all" || ip == "" {
			cmdParts = append(cmdParts, "any")
		} else {
			cmdParts = append(cmdParts, ip)
		}

		if port != "all" && port != "" {
			cmdParts = append(cmdParts, "to", "any", "port", port)
		}

		cmdParts = append(cmdParts, "comment", fmt.Sprintf("'%s'", sanitizedComment))
		return strings.Join(cmdParts, " ")
	}

	if protocol == "all" {
		// When protocol is 'all', create both TCP and UDP rules
		commands = append(commands, buildCommand("tcp"))
		commands = append(commands, buildCommand("udp"))
	} else {
		commands = append(commands, buildCommand(protocol))
	}

	return commands
}

func execUfwCommand(commands []string) error {
	for _, command := range commands {
		cmd := exec.Command("bash", "-c", command)

		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		stdoutStr := strings.TrimSpace(stdout.String())
		stderrStr := strings.TrimSpace(stderr.String())

		if stdoutStr != "" && !noCmdOut {
			common.LogDebug(fmt.Sprintf("UFW stdout: %s", stdoutStr))
		}
		if stderrStr != "" {
			if err != nil {
				common.LogError(fmt.Sprintf("UFW stderr: %s", stderrStr))
			} else if !noCmdOut {
				common.LogInfo(fmt.Sprintf("UFW status: %s", stderrStr))
			}
		}

		if err != nil {
			return fmt.Errorf("command '%s' failed: %w\nstderr: %s", command, err, stderrStr)
		}
	}
	return nil
}

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
	"github.com/rs/zerolog/log"
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
			log.Error().Msg(fmt.Sprintf("UFW execution failed: %v", err))
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

	log.Info().Msg("UFW rule application process completed.")
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

	log.Debug().Str("component", "ufwApply").Str("operation", "loadUfwConfig").Str("action", "loaded_config").Int("rule_urls_count", len(ufwConfig.RuleURLs)).Int("static_rules_count", len(ufwConfig.StaticRules)).Msg("Loaded config via common.ConfInit")

	// If detailed URL logging is still needed, ConfInit might need adjustment or
	// we might need to access the global viper instance if ConfInit uses it.
	// Assuming basic count logging is sufficient for now.

	monoDir := defaultMonoDir // Keep default monoDir definition

	log.Debug().Str("component", "ufwApply").Str("operation", "loadUfwConfig").Str("action", "loaded_config").Int("rule_urls_count", len(ufwConfig.RuleURLs)).Int("static_rules_count", len(ufwConfig.StaticRules)).Msg("Loaded config")

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
			log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "createDirectories").Str("action", "create_directory_failed").Str("directory", dir).Msg("Failed to create directory")
		} else {
			log.Debug().Str("component", "ufwApply").Str("operation", "createDirectories").Str("action", "create_directory_success").Str("directory", dir).Msg("Ensured directory exists")
		}
	}
	return nil
}

func processRuleURLs(config *Config, tmpDir string, processedFiles map[string]bool) int {
	processed := 0
	urlToFile := make(map[string]string) // Map to track URL to filename mappings
	log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "processing_rule_urls").Int("rule_urls_count", len(config.RuleURLs)).Msg("Processing rule URLs")

	// First pass: check for duplicate filenames
	for _, rule := range config.RuleURLs {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			continue
		}
		currentURL := parts[0]
		filename := filepath.Base(currentURL)
		if existingURL, exists := urlToFile[filename]; exists {
			log.Warn().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "duplicate_filename_detected").Str("filename", filename).Str("existing_url", existingURL).Str("current_url", currentURL).Msg("Duplicate filename detected")
		} else {
			urlToFile[filename] = currentURL
		}
	}

	for _, rule := range config.RuleURLs {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			log.Warn().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "invalid_rule_url_format").Str("rule", rule).Msg("Invalid rule URL format, skipping")
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
		log.Debug().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "current_rule").Str("rule_file", ruleFile).Str("rule_url", ruleURL).Msg("Processing current rule")

		tmpFile := filepath.Join(tmpDir, ruleFile+"-tmp")
		destFile := filepath.Join(config.RulesDir, ruleFile)
		rulesetFile := filepath.Join(config.RulesetDir, ruleFile)

		log.Debug().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "processing_rule").Str("rule_url", ruleURL).Str("rule_protocol", ruleProtocol).Str("rule_port", rulePort).Str("rule_description", ruleDescription).Msg("Processing rule")

		if err := downloadFile(ruleURL, tmpFile); err != nil {
			log.Error().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "download_failed").Str("rule_url", ruleURL).Msg("Failed to download rule URL")
			continue
		}
		log.Debug().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "download_success").Str("rule_url", ruleURL).Str("tmp_file", tmpFile).Msg("Downloaded rule URL to tmp file")

		needsUpdate := true
		if common.FileExists(destFile) {
			origSum, errOrig := calculateSHA256(destFile)
			newSum, errNew := calculateSHA256(tmpFile)

			if errOrig != nil {
				log.Warn().Err(errOrig).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "sha256_calculation_failed").Str("dest_file", destFile).Msg("Could not calculate SHA256 for existing file")
			} else if errNew != nil {
				log.Error().Err(errNew).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "sha256_calculation_failed").Str("tmp_file", tmpFile).Msg("Could not calculate SHA256 for downloaded file")
				os.Remove(tmpFile)
				continue
			} else {
				var portFileContent string
				if common.FileExists(rulesetFile) {
					data, err := os.ReadFile(rulesetFile)
					if err != nil {
						log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "ruleset_file_read_failed").Str("ruleset_file", rulesetFile).Msg("Could not read ruleset file")
					} else {
						portFileContent = string(data)
					}
				}

				portFileNew := fmt.Sprintf("%s %s", ruleProtocol, rulePort)
				if ruleDescription != "default" {
					portFileNew += " " + ruleDescription
				}

				if origSum == newSum && portFileContent == portFileNew {
					log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "rule_file_up_to_date").Str("rule_file", ruleFile).Msg("Rule file is up-to-date. No changes needed")
					os.Remove(tmpFile)
					needsUpdate = false
				} else {
					if origSum != newSum {
						log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "content_mismatch").Str("rule_file", ruleFile).Str("orig_sum", origSum).Str("new_sum", newSum).Msg("Content mismatch. Updating")
					}
					if portFileContent != portFileNew {
						log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "metadata_mismatch").Str("rule_file", ruleFile).Str("port_file_content", portFileContent).Str("port_file_new", portFileNew).Msg("Metadata mismatch. Updating")
					}
					if err := removeFileRules(config, destFile); err != nil {
						log.Error().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "remove_old_rules_failed").Str("dest_file", destFile).Msg("Error removing old rules. Skipping update")
						os.Remove(tmpFile)
						continue
					}
					if err := os.Remove(destFile); err != nil && !os.IsNotExist(err) {
						log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "remove_old_rule_file_failed").Str("dest_file", destFile).Msg("Could not remove old rule file")
					}
				}
			}
		} else {
			log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "new_rule_file").Str("rule_file", ruleFile).Msg("New rule file. Applying")
		}

		if needsUpdate {
			if err := os.Rename(tmpFile, destFile); err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "move_failed").Str("tmp_file", tmpFile).Str("dest_file", destFile).Msg("Failed to move tmp file to dest file")
				continue
			}
			log.Debug().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "move_success").Str("tmp_file", tmpFile).Str("dest_file", destFile).Msg("Moved tmp file to dest file")

			if err := applyFileRules(config, destFile, ruleProtocol, rulePort, ruleDescription); err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "apply_rules_failed").Str("dest_file", destFile).Msg("Error applying rules from dest file")
			}
		}

		processed++
		processedFiles[ruleFile] = true
		log.Debug().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "processed_rule").Str("rule_file", ruleFile).Str("rule_url", ruleURL).Msg("Marked rule file as processed")
	}

	log.Info().Str("component", "ufwApply").Str("operation", "processRuleURLs").Str("action", "processed_rule_urls").Int("processed", processed).Msg("Processed rule URLs")
	return processed
}

func cleanupUnusedRules(config *Config, processedFiles map[string]bool) {
	log.Info().Msg("Cleaning up unused URL-based rule files...")
	files, err := os.ReadDir(config.RulesDir)
	if err != nil {
		log.Error().Err(err).Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "read_rules_dir_failed").Str("rules_dir", config.RulesDir).Msg("Error reading rules directory")
		return
	}

	// Debug logging for processed files
	// Debug logging for processed files
	processedCount := len(processedFiles)
	processedNames := make([]string, 0, processedCount)
	for name := range processedFiles {
		processedNames = append(processedNames, name)
	}
	log.Debug().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "tracking_files").Int("processed_count", processedCount).Str("processed_names", strings.Join(processedNames, ", ")).Msg("Tracking files from config")

	// Debug logging for existing files
	existingNames := make([]string, 0)
	for _, file := range files {
		if !file.IsDir() && !strings.HasPrefix(file.Name(), "static-") {
			existingNames = append(existingNames, file.Name())
		}
	}
	log.Debug().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "found_files").Int("existing_count", len(existingNames)).Str("existing_names", strings.Join(existingNames, ", ")).Msg("Found files on disk")

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), "static-") {
			continue
		}

		if !processedFiles[file.Name()] {
			filePath := filepath.Join(config.RulesDir, file.Name())
			log.Info().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "obsolete_file").Str("file_name", file.Name()).Msg("Rule file is no longer listed in config URLs. Removing associated rules and file")

			if err := removeFileRules(config, filePath); err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "remove_rules_failed").Str("file_path", filePath).Msg("Error removing rules for obsolete file")
			}

			rulesetFile := filepath.Join(config.RulesetDir, file.Name())
			if err := os.Remove(rulesetFile); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "remove_ruleset_file_failed").Str("ruleset_file", rulesetFile).Msg("Could not remove ruleset file")
			} else if err == nil {
				log.Debug().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "remove_ruleset_file_success").Str("ruleset_file", rulesetFile).Msg("Removed ruleset file")
			}

			if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "remove_rule_file_failed").Str("file_path", filePath).Msg("Could not remove rule file")
			} else if err == nil {
				log.Debug().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "remove_rule_file_success").Str("file_path", filePath).Msg("Removed rule file")
			}
		}
	}
	log.Info().Str("component", "ufwApply").Str("operation", "cleanupUnusedRules").Str("action", "finished").Msg("Finished cleaning up unused URL-based rules")
}

func removeFileRules(config *Config, ruleContentFilePath string) error {
	ruleFileName := filepath.Base(ruleContentFilePath)
	rulesetFile := filepath.Join(config.RulesetDir, ruleFileName)

	if !common.FileExists(rulesetFile) {
		log.Warn().Str("component", "ufwApply").Str("operation", "removeFileRules").Str("action", "ruleset_file_not_found").Str("rule_file_name", ruleFileName).Str("ruleset_file", rulesetFile).Msg("Cannot remove rules for. Ruleset file not found")
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

	log.Info().Str("component", "ufwApply").Str("operation", "removeFileRules").Str("action", "removing_rules").Str("rule_content_file_path", ruleContentFilePath).Str("protocol", protocol).Str("port", port).Str("description", description).Msg("Removing UFW rules defined in")

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
				log.Info().Str("component", "ufwApply").Str("operation", "removeFileRules").Str("action", "executing_command").Str("command", cmd).Msg("Executing command")
			}
		}

		if !dryRun {
			if err := execUfwCommand(commands); err != nil {
				errMsg := fmt.Sprintf("failed to execute delete command: %v", err)
				log.Error().Str("component", "ufwApply").Str("operation", "removeFileRules").Str("action", "delete_command_failed").Str("error", errMsg).Msg("Failed to execute delete command")
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

	log.Info().Str("component", "ufwApply").Str("operation", "removeFileRules").Str("action", "finished").Str("rule_file_name", ruleFileName).Msg("Finished removing rules")
	return nil
}

func applyFileRules(config *Config, ruleContentFilePath string, protocol, port, description string) error {
	ruleFileName := filepath.Base(ruleContentFilePath)
	rulesetFile := filepath.Join(config.RulesetDir, ruleFileName)

	log.Info().Str("component", "ufwApply").Str("operation", "applyFileRules").Str("action", "applying_rules").Str("rule_content_file_path", ruleContentFilePath).Str("protocol", protocol).Str("port", port).Str("description", description).Msg("Applying UFW rules from")

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
	log.Debug().Str("component", "ufwApply").Str("operation", "applyFileRules").Str("action", "wrote_rule_definition").Str("ruleset_file", rulesetFile).Msg("Wrote rule definition to")

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
				log.Info().Str("component", "ufwApply").Str("operation", "applyFileRules").Str("action", "executing_command").Str("command", cmd).Msg("Executing command")
			}
		}

		if !dryRun {
			if err := execUfwCommand(commands); err != nil {
				errMsg := fmt.Sprintf("failed to execute allow command: %v", err)
				log.Error().Str("component", "ufwApply").Str("operation", "applyFileRules").Str("action", "allow_command_failed").Str("error", errMsg).Msg("Failed to execute allow command")
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

	log.Info().Str("component", "ufwApply").Str("operation", "applyFileRules").Str("action", "finished").Str("rule_file_name", ruleFileName).Msg("Finished applying rules")
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

	log.Info().Str("component", "ufwApply").Str("operation", "removeStaticRule").Str("action", "removing_static_rule").Str("rule_definition", ruleDefinition).Msg("Removing static rule")

	commands := buildUfwCommand("delete allow", ip, protocol, port, description)

	if !noCmdOut {
		for _, cmd := range commands {
			log.Info().Str("component", "ufwApply").Str("operation", "removeStaticRule").Str("action", "executing_command").Str("command", cmd).Msg("Executing command")
		}
	}

	if !dryRun {
		if err := execUfwCommand(commands); err != nil {
			log.Error().Err(err).Str("component", "ufwApply").Str("operation", "removeStaticRule").Str("action", "remove_static_rule_failed").Str("rule_definition", ruleDefinition).Msg("Failed to remove static rule")
			return err
		}
	}
	return nil
}

func applyStatic(config *Config) {
	log.Info().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "applying_static_rules").Int("static_rules_count", len(config.StaticRules)).Msg("Applying static rules")
	appliedCount := 0
	skippedCount := 0
	errorCount := 0

	for _, rule := range config.StaticRules {
		parts := strings.Fields(rule)
		if len(parts) < 3 {
			log.Warn().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "invalid_static_rule_format").Str("rule", rule).Msg("Invalid static rule format, skipping")
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
				log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "read_static_rule_marker_failed").Str("marker_file_path", markerFilePath).Msg("Could not read static rule marker. Re-applying rule")
			} else if string(existingContent) == rule {
				needsApply = false
				skippedCount++
			} else {
				log.Info().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "static_rule_marker_exists").Str("marker_file_path", markerFilePath).Str("existing_content", string(existingContent)).Str("rule", rule).Msg("Static rule marker exists but content differs. Removing old and re-applying")
				if err := removeStaticRule(string(existingContent)); err != nil {
					log.Error().Err(err).Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "remove_outdated_static_rule_failed").Str("existing_content", string(existingContent)).Msg("Failed to remove outdated static rule before applying update")
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
					log.Info().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "executing_command").Str("command", cmd).Msg("Executing command")
				}
			}

			if !dryRun {
				if err := execUfwCommand(commands); err != nil {
					log.Error().Err(err).Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "apply_static_rule_failed").Str("rule", rule).Msg("Failed to apply static rule")
					errorCount++
					continue
				}
			}

			err := os.WriteFile(markerFilePath, []byte(rule), 0644)
			if err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "write_static_rule_marker_failed").Str("marker_file_path", markerFilePath).Msg("Failed to write static rule marker file")
				errorCount++
			} else {
				log.Debug().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "applied_static_rule").Str("marker_file_path", markerFilePath).Msg("Applied static rule and wrote marker")
				appliedCount++
			}
		}
	}
	log.Info().Str("component", "ufwApply").Str("operation", "applyStatic").Str("action", "finished").Int("applied_count", appliedCount).Int("skipped_count", skippedCount).Int("error_count", errorCount).Msg("Finished applying static rules")
}

func checkAndRemoveStatic(config *Config) {
	log.Info().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "checking_for_obsolete_static_rules").Msg("Checking for obsolete static rules")
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
	log.Debug().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "found_configured_static_rules").Int("configured_static_rules_count", len(configuredRules)).Msg("Found configured static rules")

	dirEntries, err := os.ReadDir(config.RulesetDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "ruleset_dir_not_found").Str("ruleset_dir", config.RulesetDir).Msg("Ruleset directory does not exist. Skipping static rule cleanup")
			return
		}
		log.Error().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "read_ruleset_dir_failed").Str("ruleset_dir", config.RulesetDir).Msg("Error reading ruleset directory")
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
				log.Warn().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "read_static_rule_marker_failed").Str("marker_file_path", markerFilePath).Msg("Could not read static rule marker. Rule might be reapplied later")
				continue
			}
			if string(existingContent) != currentRuleDef {
				log.Info().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "static_rule_marker_content_differs").Str("marker_file_path", markerFilePath).Str("existing_content", string(existingContent)).Str("current_rule_def", currentRuleDef).Msg("Static rule marker content differs from config. Removing old rule")
				if err := removeStaticRule(string(existingContent)); err != nil {
					log.Error().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "remove_outdated_static_rule_failed").Str("marker_file_path", markerFilePath).Msg("Failed to remove outdated static rule from marker")
					errorCount++
				}
			}
		} else {
			log.Info().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "static_rule_marker_found").Str("marker_file_path", markerFilePath).Msg("Static rule marker found, but rule is not in config. Removing")

			ruleToRemove, err := os.ReadFile(markerFilePath)
			if err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "read_obsolete_marker_file_failed").Str("marker_file_path", markerFilePath).Msg("Could not read obsolete marker file to remove rule")
				errorCount++
				continue
			}

			if err := removeStaticRule(string(ruleToRemove)); err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "remove_obsolete_static_rule_failed").Str("rule_to_remove", string(ruleToRemove)).Msg("Failed to remove obsolete static rule")
				errorCount++
			}

			if err := os.Remove(markerFilePath); err != nil {
				log.Error().Err(err).Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "remove_obsolete_static_rule_marker_failed").Str("marker_file_path", markerFilePath).Msg("Failed to remove obsolete static rule marker")
				errorCount++
			} else {
				log.Debug().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "removed_obsolete_static_rule_marker").Str("marker_file_path", markerFilePath).Msg("Removed obsolete static rule marker")
				removedCount++
			}
		}
	}
	log.Info().Str("component", "ufwApply").Str("operation", "checkAndRemoveStatic").Str("action", "finished").Int("removed_count", removedCount).Int("error_count", errorCount).Msg("Finished checking static rules")
}

func downloadFile(url, filePath string) error {
	log.Debug().Str("component", "ufwApply").Str("operation", "downloadFile").Str("action", "downloading_file").Str("url", url).Str("file_path", filePath).Msg("Downloading file")
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
			log.Debug().Str("component", "ufwApply").Str("operation", "execUfwCommand").Str("action", "ufw_stdout").Str("stdout", stdoutStr).Msg("UFW stdout")
		}
		if stderrStr != "" {
			if err != nil {
				log.Error().Str("component", "ufwApply").Str("operation", "execUfwCommand").Str("action", "ufw_stderr").Str("stderr", stderrStr).Msg("UFW stderr")
			} else if !noCmdOut {
				log.Info().Str("component", "ufwApply").Str("operation", "execUfwCommand").Str("action", "ufw_status").Str("status", stderrStr).Msg("UFW status")
			}
		}

		if err != nil {
			return fmt.Errorf("command '%s' failed: %w\nstderr: %s", command, err, stderrStr)
		}
	}
	return nil
}

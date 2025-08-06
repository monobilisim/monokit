package pmgHealth

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/monobilisim/monokit/common"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"
)

// getExternalIP attempts to get the external IP address of the server
func getExternalIP() (string, error) {
	// Try multiple services for reliability
	services := []string{
		"https://ifconfig.co",
		"https://ipinfo.io/ip",
		"https://api.ipify.org",
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	for _, service := range services {
		resp, err := client.Get(service)
		if err != nil {
			log.Debug().Err(err).Str("service", service).Msg("Failed to get IP from service")
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			log.Debug().Int("status", resp.StatusCode).Str("service", service).Msg("Service returned non-200 status")
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Debug().Err(err).Str("service", service).Msg("Failed to read response body")
			continue
		}

		ip := strings.TrimSpace(string(body))
		// Basic IP validation
		if isValidIP(ip) {
			log.Debug().Str("ip", ip).Str("service", service).Msg("Successfully obtained external IP")
			return ip, nil
		}
	}

	return "", fmt.Errorf("failed to obtain external IP from any service")
}

// isValidIP performs basic IP address validation
func isValidIP(ip string) bool {
	// Simple IPv4 validation
	ipRegex := `^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`
	matched, _ := regexp.MatchString(ipRegex, ip)
	if !matched {
		return false
	}

	// Check each octet is valid (0-255)
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 3 {
			return false
		}
		// Check for leading zeros (except for "0")
		if len(part) > 1 && part[0] == '0' {
			return false
		}
		// Convert to int and check range
		var num int
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
			num = num*10 + int(char-'0')
		}
		if num > 255 {
			return false
		}
	}

	return true
}

// getTempAuthorizationKey gets a temporary authorization key from mxtoolbox user API
func getTempAuthorizationKey() (string, error) {
	log.Debug().Msg("Getting temporary authorization key from mxtoolbox user API")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Call the user API endpoint to get the temp auth key
	req, err := http.NewRequest("GET", "https://mxtoolbox.com/api/v1/user", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set realistic browser headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch user API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("user API returned status %d", resp.StatusCode)
	}

	// Handle gzip compression for user API response
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		log.Debug().Msg("User API response is gzip compressed, decompressing")
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to create gzip reader for user API: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Read the response body to extract the temp auth key
	body, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read user API response: %w", err)
	}

	log.Debug().Int("response_size", len(body)).Msg("Received user API response")

	// Parse the JSON response to get the TempAuthKey
	tempAuthKey, err := extractTempAuthKeyFromUserAPI(string(body))
	if err != nil {
		return "", fmt.Errorf("failed to extract temp auth key from user API: %w", err)
	}

	if tempAuthKey == "" {
		return "", fmt.Errorf("temp auth key not found in user API response")
	}

	log.Debug().Str("temp_auth_key", tempAuthKey).Msg("Successfully extracted temp authorization key from user API")
	return tempAuthKey, nil
}

// isBlacklistIgnored checks if a blacklist name should be ignored based on the configuration
func isBlacklistIgnored(blacklistName string) bool {
	// Access the global MailHealthConfig - this will need to be fixed when the config is properly passed
	// For now, we'll use a placeholder approach
	ignorelist := getIgnoreList()

	for _, ignored := range ignorelist {
		if strings.EqualFold(blacklistName, ignored) {
			return true
		}
	}
	return false
}

// getIgnoreList returns the configured ignorelist
func getIgnoreList() []string {
	// Access the global MailHealthConfig variable
	ignorelist := MailHealthConfig.Pmg.Blacklist_check.Ignorelist

	log.Debug().
		Int("ignorelist_count", len(ignorelist)).
		Strs("ignorelist", ignorelist).
		Msg("Retrieved blacklist ignorelist from configuration")

	return ignorelist
}

// UserAPIResponse represents the JSON response from mxtoolbox user API
type UserAPIResponse struct {
	TempAuthKey string `json:"TempAuthKey"`
	IsLoggedIn  bool   `json:"IsLoggedIn"`
	UserName    string `json:"UserName"`
	// Add other fields as needed, but we only care about TempAuthKey for now
}

// BlacklistCache represents cached blacklist data
type BlacklistCache struct {
	Status      BlacklistStatus `json:"status"`
	CachedAt    time.Time       `json:"cached_at"`
	NextCheckAt time.Time       `json:"next_check_at"`
}

// extractTempAuthKeyFromUserAPI extracts the temporary authorization key from user API JSON response
func extractTempAuthKeyFromUserAPI(jsonContent string) (string, error) {
	log.Debug().Msg("Parsing user API JSON response for TempAuthKey")

	var userResponse UserAPIResponse
	err := json.Unmarshal([]byte(jsonContent), &userResponse)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to parse user API JSON response")
		// Log a sample of the JSON for debugging
		sample := jsonContent
		if len(sample) > 500 {
			sample = sample[:500] + "..."
		}
		log.Debug().Str("json_sample", sample).Msg("JSON sample for debugging")
		return "", fmt.Errorf("failed to parse JSON: %w", err)
	}

	log.Debug().
		Str("temp_auth_key", userResponse.TempAuthKey).
		Bool("is_logged_in", userResponse.IsLoggedIn).
		Str("username", userResponse.UserName).
		Msg("Successfully parsed user API response")

	if userResponse.TempAuthKey == "" {
		log.Debug().Msg("TempAuthKey is empty in user API response")
		return "", fmt.Errorf("TempAuthKey not found in user API response")
	}

	return userResponse.TempAuthKey, nil
}

// getCacheFilePath returns the path to the blacklist cache file
func getCacheFilePath() string {
	return filepath.Join(common.TmpDir, "blacklist_cache.json")
}

// shouldRunBlacklistCheck determines if we should run a new blacklist check
func shouldRunBlacklistCheck() bool {
	// Check for force test environment variable
	if os.Getenv("MONOKIT_PMG_HEALTH_BLACKLIST_CHECK_TEST") == "1" {
		log.Debug().Msg("Force blacklist check enabled via environment variable")
		return true
	}

	// Check if cache file exists and is valid
	cacheFile := getCacheFilePath()
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		log.Debug().Msg("No blacklist cache file found, running check")
		return true
	}

	// Read and parse cache file
	cache, err := loadBlacklistCache()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to load blacklist cache, running check")
		return true
	}

	// Check if it's time for the next check (every 12 hours at 12:00)
	now := time.Now()
	if now.After(cache.NextCheckAt) {
		log.Debug().
			Time("next_check", cache.NextCheckAt).
			Time("current_time", now).
			Msg("Cache expired, running new blacklist check")
		return true
	}

	log.Debug().
		Time("next_check", cache.NextCheckAt).
		Time("current_time", now).
		Msg("Using cached blacklist data")
	return false
}

// loadBlacklistCache loads cached blacklist data from file
func loadBlacklistCache() (*BlacklistCache, error) {
	cacheFile := getCacheFilePath()
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cache BlacklistCache
	err = json.Unmarshal(data, &cache)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %w", err)
	}

	return &cache, nil
}

// saveBlacklistCache saves blacklist data to cache file
func saveBlacklistCache(status BlacklistStatus) error {
	now := time.Now()

	// Calculate next check time (next 12:00, either today or tomorrow)
	nextCheck := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	if now.Hour() >= 12 {
		// If it's already past 12:00, schedule for tomorrow at 12:00
		nextCheck = nextCheck.Add(24 * time.Hour)
	}

	cache := BlacklistCache{
		Status:      status,
		CachedAt:    now,
		NextCheckAt: nextCheck,
	}

	// Update status with cache info
	cache.Status.NextCheck = nextCheck.Format("2006-01-02 15:04:05")
	cache.Status.FromCache = false // This is fresh data

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	cacheFile := getCacheFilePath()
	err = os.WriteFile(cacheFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	log.Debug().
		Str("cache_file", cacheFile).
		Time("next_check", nextCheck).
		Msg("Saved blacklist data to cache")

	return nil
}

// scrapeBlacklistStatus scrapes mxtoolbox.com for blacklist information
func scrapeBlacklistStatus(ip string) ([]BlacklistEntry, error) {
	// Try mxtoolbox first
	blacklists, err := scrapeMXToolbox(ip)
	if err != nil {
		log.Error().Err(err).Str("ip", ip).Msg("MXToolbox scraping failed")
	}

	if len(blacklists) == 0 {
		log.Error().Str("ip", ip).Msg("MXToolbox returned no results")
	}

	return blacklists, nil
}

// scrapeMXToolbox attempts to use mxtoolbox API with proper authorization
func scrapeMXToolbox(ip string) ([]BlacklistEntry, error) {
	log.Debug().Str("ip", ip).Msg("Starting mxtoolbox API request for blacklist check")

	// Get a temporary authorization key by visiting the main page
	tempAuthKey, err := getTempAuthorizationKey()
	if err != nil {
		log.Debug().Err(err).Msg("Failed to get temp authorization key")
		return nil, fmt.Errorf("failed to get authorization: %w", err)
	}

	log.Debug().Str("temp_auth_key", tempAuthKey).Msg("Obtained temporary authorization key")

	// Use the actual API endpoint that mxtoolbox uses
	url := fmt.Sprintf("https://mxtoolbox.com/api/v1/Lookup?command=blacklist&argument=%s&resultIndex=1&disableRhsbl=true&format=2", ip)

	log.Debug().Str("url", url).Str("ip", ip).Msg("Querying mxtoolbox API for blacklist status")

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set realistic browser headers to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("Referer", "https://mxtoolbox.com/SuperTool.aspx")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// Add the temporary authorization header
	req.Header.Set("TempAuthorization", tempAuthKey)
	log.Debug().Str("temp_auth_key", tempAuthKey).Msg("Added TempAuthorization header to API request")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch blacklist API: %w", err)
	}
	defer resp.Body.Close()

	log.Debug().Str("ip", ip).Int("status", resp.StatusCode).Msg("Received API response from mxtoolbox")

	if resp.StatusCode != 200 {
		// Log response body for debugging
		body, _ := io.ReadAll(resp.Body)
		log.Debug().Str("ip", ip).Int("status", resp.StatusCode).Str("body", string(body)).Msg("mxtoolbox API returned non-200 status")
		return nil, fmt.Errorf("mxtoolbox API returned status %d", resp.StatusCode)
	}

	// Handle gzip compression
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		log.Debug().Msg("Response is gzip compressed, decompressing")
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read API response body: %w", err)
	}

	log.Debug().Str("ip", ip).Int("response_size", len(body)).Msg("Successfully received API response from mxtoolbox")

	return parseMXToolboxAPI(string(body))
}

// MXToolboxAPIResponse represents the JSON response from mxtoolbox API
type MXToolboxAPIResponse struct {
	HTMLValue       string   `json:"HTML_Value"`
	UID             string   `json:"UID"`
	CommandArgument string   `json:"CommandArgument"`
	Command         string   `json:"Command"`
	ResultIndex     string   `json:"ResultIndex"`
	GASetCustomVar  []string `json:"ga_setCustomVar"`
}

// parseMXToolboxAPI parses the JSON response from mxtoolbox API
func parseMXToolboxAPI(jsonData string) ([]BlacklistEntry, error) {
	log.Debug().Msg("Parsing mxtoolbox API JSON response")

	var apiResponse MXToolboxAPIResponse
	err := json.Unmarshal([]byte(jsonData), &apiResponse)
	if err != nil {
		log.Debug().Err(err).Msg("Failed to parse JSON response, trying as plain text")
		// If JSON parsing fails, try to parse as HTML directly
		return parseBlacklistHTML(jsonData)
	}

	log.Debug().
		Str("command", apiResponse.Command).
		Str("argument", apiResponse.CommandArgument).
		Str("uid", apiResponse.UID).
		Int("html_length", len(apiResponse.HTMLValue)).
		Msg("Successfully parsed mxtoolbox API response")

	// Parse the HTML content from the API response
	if apiResponse.HTMLValue == "" {
		log.Debug().Msg("API response contains no HTML content")
		return []BlacklistEntry{}, nil
	}

	return parseBlacklistHTML(apiResponse.HTMLValue)
}

// parseBlacklistHTML parses the HTML response from mxtoolbox to extract blacklist information
func parseBlacklistHTML(htmlContent string) ([]BlacklistEntry, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var blacklists []BlacklistEntry

	// Look for blacklist results in the HTML
	// MXToolbox typically shows results in a table or div structure
	// We'll look for patterns that indicate blacklist status

	// This is a simplified parser - in a real implementation you might need
	// to adjust based on the actual HTML structure of mxtoolbox
	blacklists = extractBlacklistData(doc)

	// If we couldn't parse the HTML properly, try regex fallback
	if len(blacklists) == 0 {
		log.Debug().Msg("HTML parsing returned no results, trying regex fallback")
		blacklists = extractBlacklistDataRegex(htmlContent)

		// If regex also fails, it might be a JavaScript-heavy page
		if len(blacklists) == 0 {
			log.Debug().Msg("Both HTML and regex parsing failed - page might require JavaScript")
			log.Debug().Int("html_length", len(htmlContent)).Msg("HTML content length for debugging")

			// Log a sample of the HTML content for debugging (first 500 chars)
			sample := htmlContent
			if len(sample) > 500 {
				sample = sample[:500] + "..."
			}
			log.Debug().Str("html_sample", sample).Msg("HTML content sample for debugging")
		}
	}

	log.Debug().Int("count", len(blacklists)).Msg("Extracted blacklist entries from HTML")

	// Log each blacklist that will be checked
	for _, bl := range blacklists {
		log.Debug().
			Str("blacklist", bl.Name).
			Bool("listed", bl.Listed).
			Msg("Blacklist check result parsed")
	}

	return blacklists, nil
}

// extractBlacklistData extracts blacklist data from parsed HTML
func extractBlacklistData(n *html.Node) []BlacklistEntry {
	var blacklists []BlacklistEntry

	log.Debug().Msg("Starting HTML-based blacklist extraction")

	// Parse the HTML to find table rows with blacklist results
	// Each blacklist result is in a <tr> with specific structure:
	// <td class="table-column-Status"> - contains status (OK or error icon)
	// <td class="table-column-Name"> - contains blacklist name
	// <td class="tool-blacklist-reason table-column-ReasonForListing"> - contains reason if listed

	blacklists = parseBlacklistTable(n)

	log.Debug().Int("parsed_count", len(blacklists)).Msg("HTML extraction completed")
	return blacklists
}

// parseBlacklistTable recursively parses HTML nodes to find blacklist table rows
func parseBlacklistTable(n *html.Node) []BlacklistEntry {
	var blacklists []BlacklistEntry

	// If this is a table row, check if it contains blacklist data
	if n.Type == html.ElementNode && n.Data == "tr" {
		entry := parseBlacklistRow(n)
		if entry.Name != "" && entry.CheckError != "timeout" && !isBlacklistIgnored(entry.Name) {
			blacklists = append(blacklists, entry)
			log.Debug().
				Str("blacklist", entry.Name).
				Bool("listed", entry.Listed).
				Str("error", entry.CheckError).
				Msg("Parsed blacklist entry from HTML")
		} else if entry.CheckError == "timeout" {
			log.Debug().
				Str("blacklist", entry.Name).
				Msg("Skipped blacklist entry due to timeout")
		} else if isBlacklistIgnored(entry.Name) {
			log.Debug().
				Str("blacklist", entry.Name).
				Msg("Skipped blacklist entry due to ignorelist")
		}
	}

	// Recursively parse child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		childResults := parseBlacklistTable(c)
		blacklists = append(blacklists, childResults...)
	}

	return blacklists
}

// parseBlacklistRow parses a single table row to extract blacklist information
func parseBlacklistRow(tr *html.Node) BlacklistEntry {
	entry := BlacklistEntry{}

	// Look for the specific table columns
	for td := tr.FirstChild; td != nil; td = td.NextSibling {
		if td.Type != html.ElementNode || td.Data != "td" {
			continue
		}

		// Check the class attribute to identify the column type
		class := getAttributeValue(td, "class")

		switch {
		case strings.Contains(class, "table-column-Status"):
			// Parse status - look for OK vs error indicators
			statusText := getTextContent(td)
			statusUpper := strings.ToUpper(statusText)

			// Skip TIMEOUT entries
			if strings.Contains(statusUpper, "TIMEOUT") {
				entry.CheckError = "timeout"
				log.Debug().Str("blacklist", entry.Name).Msg("Skipping blacklist entry due to timeout")
				return BlacklistEntry{} // Return empty entry to skip this one
			}

			entry.Listed = !strings.Contains(statusUpper, "OK")

			// Also check for status icons
			if img := findChildElement(td, "img"); img != nil {
				src := getAttributeValue(img, "src")
				if strings.Contains(src, "ok.png") {
					entry.Listed = false
				} else if strings.Contains(src, "error.png") || strings.Contains(src, "warning.png") {
					entry.Listed = true
				}
			}

		case strings.Contains(class, "table-column-Name"):
			// Extract blacklist name
			entry.Name = strings.TrimSpace(getTextContent(td))

		case strings.Contains(class, "tool-blacklist-reason") || strings.Contains(class, "table-column-ReasonForListing"):
			// Extract reason for listing (if any)
			reason := strings.TrimSpace(getTextContent(td))
			if reason != "" {
				entry.CheckError = reason
				entry.Listed = true // If there's a reason, it means it's listed
			}
		}
	}

	return entry
}

// Helper functions for HTML parsing

// getAttributeValue gets the value of an attribute from an HTML node
func getAttributeValue(n *html.Node, attrName string) string {
	for _, attr := range n.Attr {
		if attr.Key == attrName {
			return attr.Val
		}
	}
	return ""
}

// getTextContent extracts all text content from an HTML node and its children
func getTextContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}

	var text strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		text.WriteString(getTextContent(c))
	}
	return text.String()
}

// findChildElement finds the first child element with the specified tag name
func findChildElement(n *html.Node, tagName string) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == tagName {
			return c
		}
		// Recursively search in children
		if found := findChildElement(c, tagName); found != nil {
			return found
		}
	}
	return nil
}

// extractBlacklistDataRegex uses regex to extract blacklist information as fallback
func extractBlacklistDataRegex(htmlContent string) []BlacklistEntry {
	var blacklists []BlacklistEntry

	// Look for common patterns in mxtoolbox HTML that indicate blacklist status
	// This is a simplified approach - you might need to adjust based on actual HTML

	// Pattern to find blacklist names and their status based on mxtoolbox HTML structure
	// Look for table rows with blacklist data
	patterns := []struct {
		name    string
		pattern string
	}{
		{"table_row", `<tr><td class="table-column-Status">.*?<span class="bld_name">([^<]+)</span>.*?</tr>`},
		{"status_ok", `<img src="[^"]*ok\.png"[^>]*>`},
		{"status_error", `<img src="[^"]*(?:error|warning)\.png"[^>]*>`},
		{"blacklist_name", `<span class="bld_name">([^<]+)</span>`},
		{"reason_for_listing", `<td class="tool-blacklist-reason[^"]*">([^<]*)</td>`},
	}

	log.Debug().Int("pattern_count", len(patterns)).Msg("Starting regex-based blacklist extraction")

	// Extract blacklist names first
	namePattern := regexp.MustCompile(`<span class="bld_name">([^<]+)</span>`)
	nameMatches := namePattern.FindAllStringSubmatch(htmlContent, -1)

	log.Debug().Int("name_matches", len(nameMatches)).Msg("Found blacklist name matches")

	for _, nameMatch := range nameMatches {
		if len(nameMatch) < 2 {
			continue
		}

		blacklistName := strings.TrimSpace(nameMatch[1])
		log.Debug().Str("blacklist", blacklistName).Msg("Processing blacklist entry")

		// Skip if blacklist is in the ignorelist
		if isBlacklistIgnored(blacklistName) {
			log.Debug().Str("blacklist", blacklistName).Msg("Skipping blacklist entry due to ignorelist")
			continue
		}

		// Find the table row containing this blacklist
		rowPattern := regexp.MustCompile(`<tr[^>]*>.*?<span class="bld_name">` + regexp.QuoteMeta(blacklistName) + `</span>.*?</tr>`)
		rowMatches := rowPattern.FindAllString(htmlContent, -1)

		if len(rowMatches) > 0 {
			row := rowMatches[0]
			log.Debug().Str("blacklist", blacklistName).Str("row", row).Msg("Found table row for blacklist")

			// Skip TIMEOUT entries
			if strings.Contains(strings.ToUpper(row), "TIMEOUT") {
				log.Debug().Str("blacklist", blacklistName).Msg("Skipping blacklist entry due to timeout")
				continue
			}

			// Check if the row contains an OK status
			isOK := strings.Contains(row, "ok.png") || strings.Contains(row, "Status Ok")
			isError := strings.Contains(row, "error.png") || strings.Contains(row, "warning.png")

			// Check for reason for listing
			reasonPattern := regexp.MustCompile(`<td class="tool-blacklist-reason[^"]*">([^<]*)</td>`)
			reasonMatches := reasonPattern.FindStringSubmatch(row)

			listed := false
			checkError := ""

			if isError {
				listed = true
				log.Debug().Str("blacklist", blacklistName).Msg("IP appears to be LISTED (error icon found)")
			} else if !isOK {
				// If no OK icon and no error icon, check for reason text
				if len(reasonMatches) > 1 && strings.TrimSpace(reasonMatches[1]) != "" {
					listed = true
					checkError = strings.TrimSpace(reasonMatches[1])
					log.Debug().Str("blacklist", blacklistName).Str("reason", checkError).Msg("IP appears to be LISTED (reason found)")
				}
			} else {
				log.Debug().Str("blacklist", blacklistName).Msg("IP appears to be CLEAN (OK status found)")
			}

			blacklists = append(blacklists, BlacklistEntry{
				Name:       blacklistName,
				Listed:     listed,
				CheckError: checkError,
			})
		} else {
			log.Debug().Str("blacklist", blacklistName).Msg("Could not find table row for blacklist")
		}
	}

	log.Debug().Int("extracted_count", len(blacklists)).Msg("Regex extraction completed")

	return blacklists
}

// CheckBlacklistStatus performs the blacklist check and returns the status
func CheckBlacklistStatus(skipOutput bool) BlacklistStatus {
	log.Debug().
		Bool("skip_output", skipOutput).
		Msg("CheckBlacklistStatus called")

	status := BlacklistStatus{
		Enabled:     MailHealthConfig.Pmg.Blacklist_check.Enabled,
		CheckStatus: false,
		IsHealthy:   true,
		LastChecked: time.Now().Format("2006-01-02 15:04:05"),
	}

	log.Debug().
		Bool("enabled", status.Enabled).
		Msg("Blacklist check configuration")

	if !status.Enabled {
		log.Debug().Msg("Blacklist check is disabled")
		return status
	}

	// Check if we should use cached data or run a new check
	if !shouldRunBlacklistCheck() {
		// Load and return cached data
		cache, err := loadBlacklistCache()
		if err != nil {
			log.Error().Err(err).Msg("Failed to load cached blacklist data, running fresh check")
		} else {
			// Return cached data with updated FromCache flag
			cachedStatus := cache.Status
			cachedStatus.FromCache = true
			log.Debug().
				Time("cached_at", cache.CachedAt).
				Time("next_check", cache.NextCheckAt).
				Bool("skip_output", skipOutput).
				Msg("Returning cached blacklist data - ALARM LOGIC WILL BE SKIPPED")
			return cachedStatus
		}
	}

	// Get IP address to check
	var ipToCheck string
	if MailHealthConfig.Pmg.Blacklist_check.IP != "" {
		ipToCheck = MailHealthConfig.Pmg.Blacklist_check.IP
		log.Debug().Str("ip", ipToCheck).Msg("Using configured IP for blacklist check")
	} else {
		var err error
		ipToCheck, err = getExternalIP()
		if err != nil {
			status.CheckError = fmt.Sprintf("Failed to get external IP: %v", err)
			log.Error().Err(err).Msg("Failed to get external IP for blacklist check")
			if !skipOutput {
				common.AlarmCheckDown("blacklist_check", "Failed to get external IP for blacklist check: "+err.Error(), false, "", "")
			}
			return status
		}
		log.Debug().Str("ip", ipToCheck).Msg("Auto-detected IP for blacklist check")
	}

	status.IPAddress = ipToCheck

	// Perform the blacklist check
	blacklists, err := scrapeBlacklistStatus(ipToCheck)
	if err != nil {
		status.CheckError = fmt.Sprintf("Failed to check blacklists: %v", err)
		log.Error().Err(err).Str("ip", ipToCheck).Msg("Failed to check blacklists")
		if !skipOutput {
			common.AlarmCheckDown("blacklist_check", "Failed to check blacklists for "+ipToCheck+": "+err.Error(), false, "", "")
		}
		return status
	}

	status.CheckStatus = true
	status.Blacklists = blacklists
	status.TotalLists = len(blacklists)

	// Count ignored blacklists
	ignorelist := getIgnoreList()
	status.IgnoredCount = len(ignorelist)

	// Count how many lists the IP is on and log each result
	listedCount := 0
	for _, bl := range blacklists {
		if bl.Listed {
			listedCount++
			log.Debug().
				Str("ip", ipToCheck).
				Str("blacklist", bl.Name).
				Bool("listed", true).
				Msg("IP is LISTED on blacklist")
		} else {
			log.Debug().
				Str("ip", ipToCheck).
				Str("blacklist", bl.Name).
				Bool("listed", false).
				Msg("IP is clean on blacklist")
		}

		// Log any check errors for this specific blacklist
		if bl.CheckError != "" {
			log.Debug().
				Str("ip", ipToCheck).
				Str("blacklist", bl.Name).
				Str("error", bl.CheckError).
				Msg("Error checking blacklist")
		}
	}
	status.ListedCount = listedCount
	status.IsHealthy = listedCount == 0

	// Handle alarms
	// Note: Alarms should always be sent regardless of skipOutput (skipOutput is for UI rendering only)
	// Check for blacklists that we are LISTED on AND are not in the ignorelist
	listedNonIgnoredBlacklists := make([]string, 0)

	for _, bl := range blacklists {
		if bl.Listed && !isBlacklistIgnored(bl.Name) {
			listedNonIgnoredBlacklists = append(listedNonIgnoredBlacklists, bl.Name)
		}
	}

	log.Debug().
		Str("ip", ipToCheck).
		Int("listed_non_ignored_count", len(listedNonIgnoredBlacklists)).
		Strs("listed_non_ignored_blacklists", listedNonIgnoredBlacklists).
		Msg("Checking for blacklists we are listed on that are not in ignorelist")

	// Send alarm if we are listed on blacklists that are not in the ignorelist
	if len(listedNonIgnoredBlacklists) > 0 {
		message := fmt.Sprintf("IP %s is listed on %d blacklist(s) not in ignorelist: %s",
			ipToCheck, len(listedNonIgnoredBlacklists), strings.Join(listedNonIgnoredBlacklists, ", "))
		common.AlarmCheckDown("blacklist_not_ignored", message, true, "", "")

		log.Debug().
			Str("ip", ipToCheck).
			Int("listed_non_ignored_count", len(listedNonIgnoredBlacklists)).
			Strs("listed_non_ignored_blacklists", listedNonIgnoredBlacklists).
			Msg("Alarm sent for blacklists we are listed on that are not in ignorelist")
	} else {
		common.AlarmCheckUp("blacklist_not_ignored", fmt.Sprintf("All blacklists for IP %s are now fixed (%d lists checked)", ipToCheck, status.TotalLists), false)
	}

	log.Debug().
		Str("ip", ipToCheck).
		Int("total_lists", status.TotalLists).
		Int("listed_count", status.ListedCount).
		Bool("is_healthy", status.IsHealthy).
		Msg("Blacklist check completed")

	// Save fresh data to cache
	if err := saveBlacklistCache(status); err != nil {
		log.Error().Err(err).Msg("Failed to save blacklist data to cache")
	}

	return status
}

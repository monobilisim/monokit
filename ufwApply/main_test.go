package ufwApply

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFailedDownloadKeepsExistingRules(t *testing.T) {
	dryRun, noCmdOut = true, true
	t.Cleanup(func() { dryRun, noCmdOut = false, false })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	base := t.TempDir()
	config := &Config{
		RuleURLs:   []string{srv.URL + "/mono.txt tcp 8007"},
		RulesDir:   filepath.Join(base, "rules"),
		RulesetDir: filepath.Join(base, "ruleset"),
	}
	tmpDir := filepath.Join(base, "tmp")
	if err := createDirectories(config, tmpDir); err != nil {
		t.Fatal(err)
	}
	destFile := filepath.Join(config.RulesDir, "mono.txt")
	rulesetFile := filepath.Join(config.RulesetDir, "mono.txt")
	if err := os.WriteFile(destFile, []byte("10.0.0.0/8\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(rulesetFile, []byte("tcp 8007"), 0644); err != nil {
		t.Fatal(err)
	}

	processedFiles := make(map[string]bool)
	processRuleURLs(config, tmpDir, processedFiles)
	cleanupUnusedRules(config, processedFiles)

	for _, f := range []string{destFile, rulesetFile} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("%s removed after failed download: %v", f, err)
		}
	}
}

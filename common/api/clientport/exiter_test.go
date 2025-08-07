package clientport

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOSExiter_Structure(t *testing.T) {
	// Test that OSExiter can be instantiated
	exiter := OSExiter{}
	assert.NotNil(t, exiter)
	
	// Test that OSExiter implements the expected interface
	// (assuming there's an Exiter interface, though not visible in the provided code)
	var _ interface{ Exit(int) } = OSExiter{}
}

func TestOSExiter_Exit(t *testing.T) {
	// Since os.Exit() terminates the process, we need to test it indirectly
	// by running the test in a subprocess
	
	if os.Getenv("TEST_EXIT") == "1" {
		// This code runs in the subprocess
		exiter := OSExiter{}
		exiter.Exit(42)
		return
	}
	
	// Run the test in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestOSExiter_Exit")
	cmd.Env = append(os.Environ(), "TEST_EXIT=1")
	err := cmd.Run()
	
	// Check that the subprocess exited with the expected code
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 42, exitError.ExitCode())
	} else {
		t.Fatalf("Expected exit error, got: %v", err)
	}
}

func TestOSExiter_ExitZero(t *testing.T) {
	// Test exit with code 0
	if os.Getenv("TEST_EXIT_ZERO") == "1" {
		exiter := OSExiter{}
		exiter.Exit(0)
		return
	}
	
	cmd := exec.Command(os.Args[0], "-test.run=TestOSExiter_ExitZero")
	cmd.Env = append(os.Environ(), "TEST_EXIT_ZERO=1")
	err := cmd.Run()
	
	// Exit code 0 should result in no error
	assert.NoError(t, err)
}

func TestOSExiter_ExitOne(t *testing.T) {
	// Test exit with code 1
	if os.Getenv("TEST_EXIT_ONE") == "1" {
		exiter := OSExiter{}
		exiter.Exit(1)
		return
	}
	
	cmd := exec.Command(os.Args[0], "-test.run=TestOSExiter_ExitOne")
	cmd.Env = append(os.Environ(), "TEST_EXIT_ONE=1")
	err := cmd.Run()
	
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 1, exitError.ExitCode())
	} else {
		t.Fatalf("Expected exit error, got: %v", err)
	}
}

func TestOSExiter_ExitNegative(t *testing.T) {
	// Test exit with negative code
	if os.Getenv("TEST_EXIT_NEGATIVE") == "1" {
		exiter := OSExiter{}
		exiter.Exit(-1)
		return
	}
	
	cmd := exec.Command(os.Args[0], "-test.run=TestOSExiter_ExitNegative")
	cmd.Env = append(os.Environ(), "TEST_EXIT_NEGATIVE=1")
	err := cmd.Run()
	
	// Negative exit codes are typically converted to positive values
	assert.Error(t, err)
	if exitError, ok := err.(*exec.ExitError); ok {
		// The exact behavior of negative exit codes is platform-dependent
		// but we expect some non-zero exit code
		assert.NotEqual(t, 0, exitError.ExitCode())
	}
}

func TestOSExiter_ExitLarge(t *testing.T) {
	// Test exit with large code
	if os.Getenv("TEST_EXIT_LARGE") == "1" {
		exiter := OSExiter{}
		exiter.Exit(255)
		return
	}
	
	cmd := exec.Command(os.Args[0], "-test.run=TestOSExiter_ExitLarge")
	cmd.Env = append(os.Environ(), "TEST_EXIT_LARGE=1")
	err := cmd.Run()
	
	if exitError, ok := err.(*exec.ExitError); ok {
		assert.Equal(t, 255, exitError.ExitCode())
	} else {
		t.Fatalf("Expected exit error, got: %v", err)
	}
}

// TestOSExiter_MultipleInstances tests that multiple OSExiter instances work independently
func TestOSExiter_MultipleInstances(t *testing.T) {
	exiter1 := OSExiter{}
	exiter2 := OSExiter{}
	
	// Both should be valid instances
	assert.NotNil(t, exiter1)
	assert.NotNil(t, exiter2)
	
	// They should be equal (no state)
	assert.Equal(t, exiter1, exiter2)
}

// TestOSExiter_ZeroValue tests that zero value of OSExiter works
func TestOSExiter_ZeroValue(t *testing.T) {
	var exiter OSExiter
	
	// Zero value should be usable
	assert.NotNil(t, exiter)
	
	// We can't actually call Exit() on zero value without terminating the test,
	// but we can verify the method exists and is callable
	assert.NotPanics(t, func() {
		// This would call os.Exit if executed, but we're just checking
		// that the method signature is correct
		_ = exiter.Exit
	})
}

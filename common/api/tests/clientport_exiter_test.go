//go:build with_api

package tests

import (
	"testing"

	"github.com/monobilisim/monokit/common/api/clientport"
	"github.com/stretchr/testify/assert"
)

func TestOSExiter_InterfaceImplementation(t *testing.T) {
	// Test that OSExiter implements the Exiter interface
	var exiter clientport.Exiter = clientport.OSExiter{}
	assert.NotNil(t, exiter)
}

func TestOSExiter_Structure(t *testing.T) {
	// Test that OSExiter can be created and used
	exiter := clientport.OSExiter{}

	// We can't actually test the Exit method because it would terminate the test process
	// Instead, we verify the structure and that it implements the interface correctly
	assert.IsType(t, clientport.OSExiter{}, exiter)
}

func TestOSExiter_ZeroValue(t *testing.T) {
	// Test that zero value of OSExiter is usable
	var exiter clientport.OSExiter

	// Verify it still implements the interface
	var _ clientport.Exiter = exiter
	assert.IsType(t, clientport.OSExiter{}, exiter)
}

func TestOSExiter_InterfaceCompliance(t *testing.T) {
	// Test that OSExiter satisfies the Exiter interface contract
	exiter := clientport.OSExiter{}

	// We can't call Exit(0) as it would terminate the test
	// But we can verify the method exists and has the right signature
	// by checking interface compliance
	var _ clientport.Exiter = exiter

	// Test that we can assign it to interface variable
	var interfaceExiter clientport.Exiter = exiter
	assert.NotNil(t, interfaceExiter)
}

func TestOSExiter_MultipleInstances(t *testing.T) {
	// Test that multiple instances work correctly
	exiter1 := clientport.OSExiter{}
	exiter2 := clientport.OSExiter{}

	// Both should implement the interface
	var _ clientport.Exiter = exiter1
	var _ clientport.Exiter = exiter2

	// They should be equal (empty structs)
	assert.Equal(t, exiter1, exiter2)
}

// Note: We cannot test the actual Exit functionality because it would terminate
// the test process. In real usage, this would be tested through integration tests
// or by using mock implementations in the code that uses OSExiter.

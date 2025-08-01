package client

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetupTestServer_BasicFunctionality(t *testing.T) {
	routes := map[string]respSpec{
		"/test": {Status: http.StatusOK, Body: "test response"},
		"/api":  {Status: http.StatusCreated, Body: `{"message": "created"}`},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	// Test first route
	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "test response", string(body))

	// Test second route
	resp, err = http.Get(srv.URL + "/api")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"message": "created"}`, string(body))
}

func TestSetupTestServer_NotFoundRoute(t *testing.T) {
	routes := map[string]respSpec{
		"/existing": {Status: http.StatusOK, Body: "found"},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	// Test non-existent route
	resp, err := http.Get(srv.URL + "/nonexistent")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Contains(t, string(body), "not found")
}

func TestSetupTestServer_EmptyRoutes(t *testing.T) {
	routes := map[string]respSpec{}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	// Any route should return 404
	resp, err := http.Get(srv.URL + "/any")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestSetupTestServer_ClientConfInjection(t *testing.T) {
	// Store original ClientConf
	originalConf := ClientConf

	routes := map[string]respSpec{
		"/test": {Status: http.StatusOK, Body: "test"},
	}

	srv, restore := setupTestServer(t, routes)

	// Verify ClientConf was modified
	assert.Equal(t, srv.URL, ClientConf.URL)
	assert.NotNil(t, ClientConf.HTTPClient)
	assert.NotEqual(t, originalConf.URL, ClientConf.URL)

	// Restore and verify ClientConf is restored
	restore()
	assert.Equal(t, originalConf.URL, ClientConf.URL)
	assert.Equal(t, originalConf.HTTPClient, ClientConf.HTTPClient)
}

func TestSetupTestServer_MultipleStatusCodes(t *testing.T) {
	routes := map[string]respSpec{
		"/ok":           {Status: http.StatusOK, Body: "success"},
		"/created":      {Status: http.StatusCreated, Body: "created"},
		"/bad-request":  {Status: http.StatusBadRequest, Body: "bad request"},
		"/unauthorized": {Status: http.StatusUnauthorized, Body: "unauthorized"},
		"/server-error": {Status: http.StatusInternalServerError, Body: "server error"},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	testCases := []struct {
		path           string
		expectedStatus int
		expectedBody   string
	}{
		{"/ok", http.StatusOK, "success"},
		{"/created", http.StatusCreated, "created"},
		{"/bad-request", http.StatusBadRequest, "bad request"},
		{"/unauthorized", http.StatusUnauthorized, "unauthorized"},
		{"/server-error", http.StatusInternalServerError, "server error"},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(srv.URL + tc.path)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tc.expectedStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedBody, string(body))
		})
	}
}

func TestSetupTestServer_EmptyBody(t *testing.T) {
	routes := map[string]respSpec{
		"/empty": {Status: http.StatusNoContent, Body: ""},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	resp, err := http.Get(srv.URL + "/empty")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Empty(t, string(body))
}

func TestSetupTestServer_JSONResponse(t *testing.T) {
	routes := map[string]respSpec{
		"/json": {
			Status: http.StatusOK,
			Body:   `{"id": 1, "name": "test", "active": true}`,
		},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	resp, err := http.Get(srv.URL + "/json")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id": 1, "name": "test", "active": true}`, string(body))
}

func TestSetupTestServer_HTTPMethods(t *testing.T) {
	routes := map[string]respSpec{
		"/endpoint": {Status: http.StatusOK, Body: "method response"},
	}

	srv, restore := setupTestServer(t, routes)
	defer restore()

	// Test different HTTP methods - the test server responds to all methods
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, srv.URL+"/endpoint", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, "method response", string(body))
		})
	}
}

func TestSetupTestServer_CleanupFunction(t *testing.T) {
	originalConf := ClientConf
	routes := map[string]respSpec{
		"/test": {Status: http.StatusOK, Body: "test"},
	}

	srv, restore := setupTestServer(t, routes)

	// Verify server is running
	resp, err := http.Get(srv.URL + "/test")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Call restore manually
	restore()

	// Verify ClientConf is restored
	assert.Equal(t, originalConf, ClientConf)

	// Verify server is closed (this might not always work reliably in tests)
	// We can't easily test server closure without potentially flaky tests
}

func TestSetupTestServer_TCleanupIntegration(t *testing.T) {
	// This test verifies that t.Cleanup is properly called
	originalConf := ClientConf
	routes := map[string]respSpec{
		"/test": {Status: http.StatusOK, Body: "test"},
	}

	// Create a subtest to verify cleanup behavior
	t.Run("subtest", func(t *testing.T) {
		srv, _ := setupTestServer(t, routes)

		// Verify ClientConf was modified
		assert.NotEqual(t, originalConf.URL, ClientConf.URL)
		assert.Equal(t, srv.URL, ClientConf.URL)

		// When this subtest ends, t.Cleanup should restore ClientConf
	})

	// After subtest completes, ClientConf should be restored
	assert.Equal(t, originalConf, ClientConf)
}

func TestRespSpec_Structure(t *testing.T) {
	// Test that respSpec struct works as expected
	spec := respSpec{
		Status: http.StatusCreated,
		Body:   "test body",
	}

	assert.Equal(t, http.StatusCreated, spec.Status)
	assert.Equal(t, "test body", spec.Body)

	// Test zero values
	var zeroSpec respSpec
	assert.Equal(t, 0, zeroSpec.Status)
	assert.Equal(t, "", zeroSpec.Body)
}

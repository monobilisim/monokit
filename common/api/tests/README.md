# Monokit API Unit Tests

This directory contains unit tests for the Monokit API.

## Running Tests

To run all API tests:

```bash
cd /path/to/monokit
go test ./common/api/tests -tags=with_api
```

To run a specific test file:

```bash
go test ./common/api/tests -tags=with_api -run TestKeycloak
```

To run a specific test function:

```bash
go test ./common/api/tests -tags=with_api -run TestHandleSSOLogin
```

## Coverage

To run tests with coverage:

```bash
go test ./common/api/tests -tags=with_api -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## Adding New Tests

When adding new tests, follow these patterns:

1. Use `SetupTestDB` to create a test database
2. Use `CleanupTestDB` to dispose of the database when done
3. For API handlers, use `CreateRequestContext` to create a test request
4. For tests requiring authentication, use `AuthorizeContext` with a test user

For Keycloak tests:
1. Use `setupKeycloakConfig()` to configure Keycloak settings
2. Use `setupMockJWKS()` to set up token validation
3. Use `createMockToken()` to create test tokens 
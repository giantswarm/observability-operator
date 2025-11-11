# Alertmanager Routes Integration Tests

Go integration tests that verify Alertmanager routing configurations by sending test alerts and checking that they reach the right receivers (Slack, OpsGenie, webhooks, etc.).

## Table of Contents

- [Overview](#overview)
- [How to use it](#how-to-use-it)
- [Running Tests](#running-tests)
- [Directory Structure](#directory-structure)
- [How It Works](#how-it-works)
- [Writing Tests](#writing-tests)
- [Troubleshooting](#troubleshooting)

## Overview

## How to use it

Both unit and integration tests for Alertmanager routing work in a similar fashion and live in the same sub directories under `tests/alertmanager-routes/`.

- Unit tests are represented by the `.bats files`. These tests validate routing logic without needing a running Alertmanager instance.
- Integration tests are represented by the `_test.go` files. These tests deploy a real Alertmanager and verify the notifications sent.

Each test takes 2 inputs:

- Helm chart values, which are used to generate the Alertmanager configuration
- A set of test alerts to be sent, which are defined as a set of labels

## Running the Tests

### Prerequisites

1. **Docker** and **Kind** (for integration tests)
2. **Helm**

`make` targets for individual tests take the directory name as the last value

### Running Integration Tests

```bash
# Run all alertmanager integration tests
make test-alertmanager-integrations

# Run alertmanager integration test for heartbeat
make test-alertmanager-integration-heartbeat
```

### Running Unit Tests

```bash
# Run all unit tests
make tests-alertmanager-routes

# Run unit test for heartbeat
make tests-alertmanager-routes-heartbeat

# Run unit test with verbose output
# Note that false positive might occur when using VERBOSE
make tests-alertmanager-routes-heartbeat VERBOSE=1
```

### Cleanup

Clean up test environment and generated files:

```bash
# Clean everything
make clean

# Remove Kind cluster and generated files
make test-alertmanager-integration-clean

# Remove only generated config files
make tests-alertmanager-routes-clean
```

### Debugging Tests

View recorded HTTP requests:

```bash
# Check the request log file (created during test execution)
cat tests/alertmanager-routes/heartbeat/integration_test_requests_TestHeartbeat.log

```

The log file contains all HTTP requests received during the test, including:
- HTTP method and URL
- Headers
- Request body

View generated Alertmanager configuration

```bash
ls tests/alertmanager-routes/heartbeat/alertmanager-config
```

## Directory Structure

```
alertmanager-routes/
├── README.md                                       # This file
├── helper.go                                       # Integration test runner
├── helper_alertmanager_client.go                   # Integration test Alertmanager API client
├── helper_http_receiver.go                         # Integration test HTTP request recorder/proxy
├── helper_expectation.go                           # Integration test assertion/validation logic
├── helper.bash                                     # Unit test helpers
│
├── default/                                        # Test case: default routing
│   ├── default_test.go                             # Integration tests
│   ├── default.bats                                # Unit tests
│   ├── chart-values.yaml                           # Input configuration values
│   ├── integration_test_requests_TestHeartbeat.log # Integration test Alertmanager request log
│   └── alertmanager-config/                        # Output configuration (generated for each test)
│       ├── alertmanager.yaml                       # Alertmanager configuration
│       ├── notification-template.tmpl              # Go template for notifications
│       └── url-template.tmpl                       # Go template for URLs
│
├── heartbeat/                                      # Test case: heartbeat alerts
│   ├── heartbeat_test.go
│   ├── heartbeat.bats
│   ├── chart-values.yaml
│   └── alertmanager-config/
│       └── ...
│
└── [other test cases]/
```

## Writing New Tests

### Integration TestCase Definition

```go
TestCase: helper.TestCase{
    Alert: helper.Alert{
        Name: "AlertName",           // Required: alert name (set as alertname label)
        Labels: map[string]string{   //           additional labels
            "key": "value",
        },
    },
    Expectation: helper.Expectation{
        URL: "https://example.com/webhook",  // Required: URL to match (substring)
        Headers: map[string]string{          // Optional: headers to verify (exact match)
            "Content-Type": "application/json",
        },
        BodyParts: []string{                 // Optional: body content to verify (substring are ANDED)
            `"alertname":"MyAlert"`,
            `"severity":"page"`,
        },
        Negate: false,                       // Optional: if true, asserts this should NOT match
    }
}
```

### Unit test case definition


```bash
@test "test case" {
  run amtool alertname=AlertName key=value  # Set of alert labels
  assert_line --partial receiver_name       # Name of the receiver. More sophisticated assertions can be built using bats-assert.
}
```

## Integration architecture

These tests work by:
1. Starting a test HTTP server that captures all outgoing notifications
2. Uploading your Alertmanager config to Mimir
3. Sending test alerts
4. Verifying the expected notifications were sent

No actual external services (Slack, OpsGenie, etc.) are needed - everything is intercepted by the test server.

```
┌─────────────┐
│  Go Test    │
│             │
│  1. Upload  │──────────────┐
│     Config  │              │
│             │              ▼
│  2. Send    │         ┌────────────────┐
│     Alerts  │────────▶│ Mimir          │
│             │         │ Alertmanager   │
│  3. Wait    │         └────────────────┘
│             │              │ Notifications
│  4. Assert  │              │ (via HTTP proxy)
│     Results │              │
└─────────────┘              ▼
       ▲              ┌──────────────┐
       │              │ HTTP         │
       └──────────────│ Receiver     │
          Records     │ (Test Proxy) │
                      └──────────────┘
```

## How the Tests Work

1. **Alertmanager Client** (`helper_alertmanager_client.go`): Manages communication with Mimir Alertmanager
   - Uploads configuration with modified proxy settings
     - Set `proxy_url` to route all HTTP traffic through the test receiver
     - Set `insecure_skip_verify: true` to allow self-signed certificates
     - Override wait/repeat intervals to speed up tests
   - Sends test alerts
   - Waits for configuration propagation

2. **HTTP Receiver** (`helper_http_receiver.go`): Test server that intercepts notifications
   - Acts as HTTP/HTTPS proxy for Alertmanager notifications
     - Alertmanager sends `CONNECT` request to the HTTP receiver
     - HTTP receiver establishes connection to its own HTTPS server
     - TLS traffic is proxied through, allowing inspection of the request
   - Records all incoming HTTP requests

3. **Expectation Matcher** (`helper_expectation.go`): Validates received notifications
   - Matches URLs, headers, and body content
   - Supports positive and negative assertions

This approach allows the tests to:
- Inspect all notification requests (including HTTPS)
- Verify request content without modifying Alertmanager
- Test without requiring actual external services

## Troubleshooting

For most test failures, reviewing request logs and expectation to see which requests were received is enough to identify the issue. For other failures, see below for common symptoms and solutions.

### No Requests Received

**Symptoms**: Test fails with "no Alertmanager notifications received"

**Possible Causes**:
1. Alertmanager configuration not propagated
   - **Solution**: Increase `--alertmanager-config-wait-timeout`

2. Routing configuration doesn't match alert labels
   - **Solution**: Review `alertmanager.yaml` routes and alert labels
   - **Debug**: Use bats unit tests to test routing

3. Wait time too short
   - **Solution**: Increase wait time in test (`RunAlertmanagerIntegrationTest` last parameter)

### Assertion Failures

**Symptoms**: Test fails with "assertion failed for expectation"

**Possible Causes**:
1. URL doesn't match
   - **Debug**: Check recorded requests in `integration_test_requests_*.log`
   - **Solution**: Verify expectation URL is a substring of actual URL

2. Body parts don't match
   - **Debug**: Review request body in log file
   - **Solution**: Adjust `BodyParts` to match actual notification format

3. Headers don't match
   - **Debug**: Check headers in log file
   - **Solution**: Remove header expectations or adjust expected values

- `no Alertmanager notifications received`

1. Use unit tests to ensure your combination of alert labels and routing config matches as expected
2. Inspect Alertmanager logs with `kubectl logs deploy/mimir-alertmanager`
3. The **Solution** is probably to start from scratch using `make test-alertmanager-integration-clean`

- `Error while proxying request`

This is a known issue with the HTTP proxy setup. But there's not solution for this apart from re-running the test.

- `panic: test timed out after`

This indicates that the test took too long to complete. The **Solution** is to increase the timeout in the `INTEGRATION_TEST_FLAGS` in the `Makefile.custom.mk` file

## Best Practices

1. **Wait Times**: Use appropriate wait times (typically 30s) to allow Alertmanager to process alerts

2. **Negative Tests**: Include negative assertions to verify alerts are NOT sent when they shouldn't be

3. **Comprehensive Coverage**: Test both positive cases (notification sent) and negative cases (notification not sent)

4. **Configuration Realism**: Use realistic Alertmanager configurations that match production setups

## Related Documentation

- [Alertmanager Configuration](https://prometheus.io/docs/alerting/latest/configuration/)
- [bats-assert](https://github.com/bats-core/bats-assert#usage)
- [Helm chart values](https://github.com/giantswarm/mimir-app/blob/main/helm/mimir/values.yaml)

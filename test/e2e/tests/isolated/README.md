# Isolated or Sequential Tests

This directory contains the E2E tests that require changes in shared resources. Due to which they cannot run in parallel with other tests. They have to run sequentially and revert config changes as a "post-test" step which is covered by "finally" block in the tests.

For example, testing LoadBalancer Class feature requires isolation from other tests otherwise rest of the tests will fail.

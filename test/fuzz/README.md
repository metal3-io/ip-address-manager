# Fuzzing

## Running Fuzz Tests

### Quick Start with Makefile

The easiest way to run fuzz tests is using the Makefile targets:

```bash
# Run fuzz tests as regression tests (using seed corpus only, fast)
make fuzz

# Run all fuzz tests with fuzzing enabled (default: 30 seconds)
make fuzz-run

# Run all fuzz tests for custom duration (e.g., 5 minutes)
make fuzz-run FUZZ_TIME=5m

# Run a specific fuzz test (default: 30 seconds)
make fuzz-run FUZZ_TARGET=FuzzNewIPPoolManager

# Run a specific fuzz test with custom duration
make fuzz-run FUZZ_TARGET=FuzzIPClaimToIPPool FUZZ_TIME=2m
```

### Available Fuzz Targets

- `FuzzNewIPPoolManager` - Tests IPPool manager creation and finalizers
- `FuzzIPClaimToIPPool` - Tests IPClaim to IPPool mapping
- `FuzzFilterAndContains` - Tests string filtering utilities
- `FuzzStringSliceOperations` - Tests string slice operations

### Manual Execution

You can also run fuzz tests directly with `go test`:

```bash
cd test/fuzz

# Run all fuzz tests for 30 seconds each
go test -fuzz=. -fuzztime=30s

# Run a specific fuzz test
go test -fuzz=FuzzNewIPPoolManager -fuzztime=1m

# Run for specific number of iterations
go test -fuzz=FuzzFilterAndContains -fuzztime=10000x
```

## Resources

- [Go Fuzzing Documentation](https://go.dev/doc/fuzz/)
- [Go Fuzzing Tutorial](https://go.dev/security/fuzz/)

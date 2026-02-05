# Fuzzing

 Fuzzing or fuzz testing is an automated software testing technique that
 involves providing invalid, unexpected, or random data as inputs to a
 computer program. A fuzzing target is defined for running the tests.
 These targets are defined in the test/fuzz directory and run using
 the instructions given below:

- Install the package from <https://github.com/mdempsky/go114-fuzz-build>
  using the following command. This tool uses libfuzzer for fuzzing:

    ```bash
    go install github.com/mdempsky/go114-fuzz-build@latest
    # Ensure $HOME/go/bin is in your PATH
    export PATH=$PATH:$HOME/go/bin
    ```

   The following are the important steps to run fuzzing tests

    a.  Run below command to build the file for the function by giving its location.

    ```bash
    mkdir output
    cd output
    go114-fuzz-build -o yaml_FuzzNewIPPoolManager.a -func FuzzNewIPPoolManager ../test/fuzz/
    ```

    b. Run below command to make C binary files for the same function.

    ```bash
    clang -o yaml_FuzzNewIPPoolManager  yaml_FuzzNewIPPoolManager.a -fsanitize=fuzzer
    ```

    c. Run the fuzzer as below

    ```bash
    ./yaml_FuzzNewIPPoolManager
    ```

- Execution: Here is an example for execution of some test outputs:

    ```text
    [63 32 45 58 119 50 9 119 50 9 119 50 9 45 58
    50 9 119 9 119 50 9 91 13 45 58]
    &{TypeMeta:{Kind: APIVersion:}
    ObjectMeta:{Name: GenerateName: Namespace: SelfLink: UID:
    ResourceVersion: Generation:0
    CreationTimestamp:0001-01-01 00:00:00 +0000 UTC DeletionTimestamp:<nil> DeletionGracePeriodSeconds:<nil>
    Labels:map[] Annotations:map[] OwnerReferences:[]
    Finalizers:[] ManagedFields:[]} Spec:{ClusterName:<nil> Pools:[]
    PreAllocations:map[] Prefix:0 Gateway:<nil> DNSServers:[]
    NamePrefix:} Status:{LastUpdated:<nil> Allocations:map[]}}
    #1028919 REDUCE cov: 4365 ft: 18232 corp:
    2348/250Kb lim: 1070 exec/s: 3535 rss: 463Mb L:
    118/1060 MS: 1 EraseBytes-
    ```

    Here first line contains the byte array provided by libfuzzer to the
    function we then typecast it to our struct of IPPool using JSON.
    Unmarshal and then run the function. It keeps on running
    until it finds a bug.

## Available Fuzzing Targets

The following fuzzing targets are available in this directory:

1. **FuzzNewIPPoolManager** - Tests IPPool manager creation and finalizer operations
   - File: `ippool_manager_fuzzer.go`
   - Tests: Manager creation, SetFinalizer, UnsetFinalizer

1. **FuzzIPClaimToIPPool** - Tests IPClaim to IPPool reconciliation mapping
   - File: `ippool_controller_fuzzer.go`
   - Tests: Controller reconciliation request mapping

1. **FuzzFilterAndContains** - Tests string list utility functions
   - File: `utils_fuzzer.go`
   - Tests: Filter and Contains functions with edge cases

1. **FuzzStringSliceOperations** - Tests string slice operations
   - File: `utils_fuzzer.go`
   - Tests: Multiple filter operations, empty list handling

To run a specific fuzzer, replace `FuzzNewIPPoolManager` with the target function
name in the commands above.

## Example: Running the Utils Fuzzer

To run the new `FuzzFilterAndContains` fuzzer:

```bash
mkdir -p output
cd output
go114-fuzz-build -o utils_FuzzFilterAndContains.a -func FuzzFilterAndContains ../test/fuzz/
clang -o utils_FuzzFilterAndContains utils_FuzzFilterAndContains.a -fsanitize=fuzzer

# Run for 60 seconds
./utils_FuzzFilterAndContains -max_total_time=60

# Or run indefinitely until crash found
./utils_FuzzFilterAndContains
```

You can also run basic smoke tests without libfuzzer:

```bash
cd test/fuzz
go test -v -run TestFuzz
```

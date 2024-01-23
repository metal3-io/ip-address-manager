# Fuzzing

 Fuzzing or fuzz testing is an automated software testing technique that
 involves providing invalid, unexpected, or random data as inputs to a
 computer program. A fuzzing target is defined for running the tests.
 These targets are defined in the test/fuzz directory and run using
 the instructions given below:

- Install the package from <https://github.com/mdempsky/go114-fuzz-build>
  using go get command. This is the link for the fuzzing tool that uses
  libfuzzer for its implementation purposes. I set it up and made a sample
  function to run the fuzzing tests. In these steps following are the important steps

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

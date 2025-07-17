# deep
Support for doing deep copies of (almost all) Go types.

This is a from scratch implementation of the ideas from https://github.com/barkimedes/go-deepcopy (which, unfortunatelly appears to be dead) but it is faster, simpler to use for callers and supports copying unexported fields.

It should support most Go types. Specificaly, it does not support functions, channels and unsafe.Pointers unless they are nil. Also it might have weird interactions with structs that include any synchronization primitives (mutexes, for example. They should still be copied but if they are usable after that is left as an exercise to the reader).

One possible usage scenario would be, for example, to negate the use of the deepcopy-gen tool described in [Kubernetes code generation](https://www.redhat.com/en/blog/kubernetes-deep-dive-code-generation-customresources). For any type T, the DeepEqual method can be implemented more or less like this:

```
func (t* T) DeepCopy() *T {
        return deep.MustCopy(t) // This would panic on errors.
}
```

## Benchmarks

| Benchmark                          | Iterations | Time           | Bytes Allocated | Allocations      |
|------------------------------------|------------|----------------|-----------------|------------------|
| **BenchmarkCopy_Deep-16**          | **830553** | **1273 ns/op** | **1264 B/op**   | **21 allocs/op** |
| BenchmarkCopy_DeepCopy-16 (1)      | 458072     | 2466 ns/op     | 1912 B/op       | 50 allocs/op     |
| BenchmarkCopy_CopyStructure-16 (2) | 149685     | 7836 ns/op     | 6392 B/op       | 168 allocs/op    |
| BenchmarkCopy_Clone-16 (3)         | 510760     | 2188 ns/op     | 1656 B/op       | 22 allocs/op     |

(1) https://github.com/barkimedes/go-deepcopy (does not support unexported fields)

(2) https://github.com/mitchellh/copystructure (does not support cycles)

(3) https://github.com/huandu/go-clone

# Dingo

[Dingo](docs/language.md) is a statically typed and compiled programming language with easy interop from and to C. The compiler is written in Go and uses LLVM for machine code generation.

This is a hobby project intended for learning and experimentation.

## Docs

[Docs](docs/language.md) and [grammar](docs/grammar.md).

## Examples

[Examples](examples) and [lib](std).

## Installation

Ensure Go 1.6 or above and subversion are installed, and that GOPATH is properly set.

Clone and install LLVM 6.0 and Go bindings.

```none
$ GOLLVM="$GOPATH/src/llvm.org/llvm"
$
$ svn co http://llvm.org/svn/llvm-project/llvm/tags/RELEASE_600/final $GOLLVM
...
$ GOLLVM/bindings/go/build.sh -DCMAKE_BUILD_TYPE=Release -DLLVM_TARGETS_TO_BUILD=host
...
$ go install llvm.org/llvm/bindings/go/llvm
...
```

Build and install compiler and test tool.

```none
$ go install github.com/cjo5/dingo/cmd/dgc
...
$ go install github.com/cjo5/dingo/cmd/dgc-test
...
```

## Usage

Compile and run program

```none
$ dgc examples/hello.dg
$
$ ./dgexe
Hello, world!
```

Run single test

```none
$ dgc-test -test test/math.dg
test 1/1 math ... ok

ok: 1/1 skip: 0 fail: 0 bad: 0
```

Run all tests

```none
$ dgc-test -manifest test/manifest.json
...
```

# Dingo

Dingo is a statically typed and compiled programming language focused on concise syntax, fast build times, full memory control, and easy interop from and to C. 

## Installation
Ensure Go 1.6 or above and subversion are installed, and that GOPATH is properly set. 

Clone and install LLVM 6.0 and Go bindings.
```
$ GOLLVM="$GOPATH/src/llvm.org/llvm"
$ svn co http://llvm.org/svn/llvm-project/llvm/tags/RELEASE_600/final $GOLLVM
$ $GOLLVM/bindings/go/build.sh -DCMAKE_BUILD_TYPE=Release -DLLVM_TARGETS_TO_BUILD=host
$ go install llvm.org/llvm/bindings/go/llvm
```

Build and install compiler and test tool.
```
$ go install github.com/cjo5/dingo/cmd/dgc
$ go install github.com/cjo5/dingo/cmd/dgc-test
```

## Usage
Compile and run program.
```
$ dgc examples/hello.dg
$ ./dgexe
Hello, world!
```

Run single test.
```
$ dgc-test -test test/math.dg
test 1/1 math ... ok

ok: 1/1 skip: 0 fail: 0 bad: 0
```

Run all tests.
```
$ dgc-test -manifest test/manifest.json
```

## Examples
See [examples](examples) for code samples.

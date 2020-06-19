# Dingo

[Dingo](docs/language.md) is a statically typed and compiled programming language with easy interop from and to C. The compiler is written in Go and uses LLVM for machine code generation.

This is a hobby project intended for learning and experimentation.

## Docs

[Docs](docs/language.md) and [grammar](docs/grammar.md).

## Examples

[Examples](examples) and [lib](std).

## Installation

Ensure Go 1.14 or a later version is installed.

Set environment variables and create directories.

```none
$ export COMP_DIR="$PWD/dingo/compiler"
$ export LLVM_DIR="$PWD/dingo/llvm10"
$ mkdir -p "$COMP_DIR" "$LLVM_DIR"
$
```

Clone repository and LLVM 10.

```none
$ git clone https://github.com/cjo5/dingo.git "$COMP_DIR"
...
$ git clone -b llvmorg-10.0.0 https://github.com/llvm/llvm-project.git "$LLVM_DIR"
...
```

Build LLVM, compiler, and test tool.

```none
$ cd "$COMP_DIR"
$ scripts/build_llvm.sh
...
$ go build -o dgc cmd/dgc/main.go
$ go build -o dgc-test cmd/dgc-test/main.go
$
```

## Dynamic LLVM Libraries

Include the path to the dynamic LLVM libraries before running the compiler or test-tool. The LLVM-LIBS file is created by ```scripts/build_llvm.sh```.

```none
$ export LD_LIBRARY_PATH="$LD_LIBRARY_PATH:$(cat LLVM-LIBS)"
$
```

## Usage

Compile and run program.

```none
$ ./dgc examples/hello.dg
$ ./dgexe
Hello, world!
```

Run single test.

```none
$ ./dgc-test -test test/math.dg
test 1/1 math ... ok

ok: 1/1 skip: 0 fail: 0 bad: 0
```

Run all tests.

```none
$ ./dgc-test -manifest test/manifest.json
...
```

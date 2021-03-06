# The Dingo Language

## Index

- [Comments](#comments)
- [Semicolons](#semicolons)
- [Modules](#modules)
- [Include](#include)
- [Use](#use)
- [Variables](#variables)
- [Typealias](#typealias)
- [Functions](#functions)
- [References / Pointers](#references--pointers)
- [Arrays](#arrays)
- [Slices](#slices)
- [Structs](#structs)
- [Typeof](#typeof)
- [Type Casting](#type-casting)
- [If](#if)
- [For / While](#for--while)
- [Defer](#defer)
- [Sizeof](#sizeof)
- [Memory Management](#memory-management)
- [C](#c)
- [Strings](#strings)
- [Booleans](#booleans)
- [Numbers](#numbers)
- [Basic Operators](#other-operators)
- [Assignments](#assignments)
- [Operator Precedence](#operator-precedence)
- [Grammar](grammar.md)
- [Naming Conventions](#naming-conventions)
- [Keywords](#keywords)

## Comments

```rust
// Single line comment.

/*
    Mutiline comment.
    Another comment.
    /*
        Nested comment.
    */
*/
```

## Semicolons

Semicolons work in a similar way as in Go. That is, the grammar and parser assume that statements are terminated with semicolons; however, the lexer automatically inserts a semicolon in the token stream at the end of a line if it sees a token that can terminate a statement. See [here](https://github.com/cjo5/dingo/blob/eb389e67264d1fdb209ead4be8d8e9e1c489b8af/internal/frontend/lex.go#L165) for the exact tokens that the lexer checks for.

## Modules

Modules provide a way to structure code in a hierarchical namespace. Declarations at the top of a file, outside any module, are implicitly defined in a module with name "" -- the root module. An identifier´s Fully Qualified Name (FQN) is the path from the root module to the identifier, with each module name separated by ```::```.

```rust
module foo {             // FQN: foo
    var bar: i32         // FQN: foo::bar
    pub module baz {     // FQN: foo::baz
        pub var qux: i32 // FQN: foo::baz::qux
    }
}
```

Name lookups inside a module are relative by default, and parents are not automatically searched. A module can access the parent scope with ```up``` (not a keyword), which is a short hand way to to do an absolute name lookup using the parent's FQN. Prefix the FQN with ```::``` to do an absolute name lookup. Module navigation is similar to file system navigation on the command line, with ```up``` as ```..``` and ```::``` as ```/```.

```rust
fun hello() {}

module foo {
    fun bar() {
        hello()     // error, hello in parent
        ::hello()   // ok
        baz::quux() // ok
    }
    module baz {
        fun qux() {
            bar()           // error, bar in parent
            quux()          // ok
            ::foo::bar()    // ok
            up::bar()       // ok
            up::up::hello() // ok
        }
        fun quux() {}
    }
}
```

A Compilation Unit (CUnit) is defined to be a file plus all its included files. A set of CUnits being compiled is referred to as a Compilation Context (CContext). Modules and declarations in the same CUnit can be accessed as previously described. To access modules or declarations in a different CUnit, the module must first be imported. The CUnit where the module is defined must be part of the CContext.

Modules and top-level declarations are either public ```pub``` or private ```priv``` (default). The parent visibility does not affect the visibility of a module. Public modules must be unique across a CContext and private modules must be unique within a CUnit. Only public modules are allowed to be imported.

a.dg:

```rust
module foo {
    import baz // ok
    import qux // error, not public

    fun hello() {
        ::bar::hello()
        baz::hidden() // error, not public
        baz::hello()  // ok
        qux::hello()  // error, not defined
    }
}

module bar {
    fun hello{}
}
```

b.dg:

```rust
pub module baz {
    fun hidden()
    pub fun hello{}
}

module qux {
    fun hidden()
    pub fun hello{}
}
```

## Include

A file can be included inside another file and module. This is equivalent to replacing the include statement with the included file's content. The FQN of every identifier in the included file is prefixed with the FQN of the module where it's included. The include path is either absolute or relative to the file with the include statement. Files are included in a breadth-first search manner and are loaded automatically during the parsing phase.

a.dg:

```rust
var bar: i32
module baz {
    var qux: i32
}
```

b.dg:

```rust
module foo {
    include "a.dg"
    // The identifiers in a.dg are brought in to the foo module.
    // FQN: foo::bar
    // FQN: foo::baz
    // FQN: foo::baz::qux
}
```

## Use

Any scope lookup can be used with ```use``` to bring the final item in the lookup into the current scope.

```rust
module foo {
    fun bar() {}
    fun qux() {}
    module baz {
        fun qux() {}
        var quux: i32
    }
}

use foo::qux
use baz_qux = foo::baz::qux

fun hello() {
    qux()
    baz_qux()
    use foo::bar
    bar()
    use q = foo::baz::quux
    q++
}

```

## Variables

```rust
var a: i32
var b: i32 = 1
var c = 2
val d: i32
val e: i32 = 1
val f = 2
```

```var``` defines a mutable variable and ```val``` defines an immutable value. The type can be omitted when there is an initializer. The variable/value is assigned a default value if there is no initializer.

## Typealias

```rust
typealias Id = i32
```

Create an alias for a type. The two types are equivalent, and either type can be substituted for the other.

## Functions

```rust
fun say_hello() {
    println("hello")
}

fun add(a: i32, b: i32) i32 {
    return a + b
}

fun inc(var a: i32) i32 {
    a += 1
    return a
}

say_hello()
add(a: 5, b: 10)    // named arguments
add(6, 11)          // positional arguments
```

No return type means the function has no return value. Function calls support named arguments in arbitrary order. There can be no positional arguments after a named argument.
Function parameters are immutable by default, but can be made mutable by preceeding the name with 'var'.

Functions can be used as values and also defined inline (function literals). Though note that function literals are not closures and do not have access to variables in the enclosing scope.

```rust
val add_val: fun (i32, i32) i32 = add
val sub = fun(a: i32, b: i32) i32 {
    return a - b
}
```

## References / Pointers

```rust
var a: i32 = 5
val b: &i32 = &a         // immutable reference
val c: &var i32 = &var a // mutable reference
val d: &i32 = null       // default value

// dereference
b[] = 5 // error, b is an immutable reference
c[] = 9 // ok

val e = 6
val f = &var e // error, cannot take a mutable reference to an immutable value
```

## Arrays

```rust
var a: [i32:5] = [i32](1, 2, 3, 4, 5) // allocated on the stack
len(a) // length of array
```

## Slices

```rust
var a = [i32](1, 2, 3, 4, 5)
val b: &[i32] = &a[1:3]          // immmutable slice
val c: &var [i32] = &var a[:3]   // mutable slice
val d: &[i32] = null             // default value

len(c) // length of slice
```

## Structs

```rust
struct Foo {
    // fields

    val a: i32      // immutable
    val b: i32      // immutable
    var count: i32  // mutable

    // methods

    // equivalent to self: &Self
    fun add(&Self) i32 {
        return self.a + self.b
    }

    // equivalent to self: &Foo
    fun sub(self: &Self) i32 {
        return self.a - self.b
    }

    // the struct type can be specified explicitly
    fun mul(self: &Foo) i32 {
        return f.a*f.b
    }

    // the parameter can have any custom name
    fun div(f: &Self) i32 {
        return f.a/f.b
    }

    // mutable parameter
    fun inc(&var Self) {
        self.count++
    }

    fun set(&var Self, a: i32, b: i32) {
        self.a = a
        self.b = b
    }

    fun say_hello() {
        println("hello")
    }
}

// allocated on the stack
var f1 = Foo(a: 5, b: 9)    // named arguments
val f2 = Foo(6, 10)         // positional arguments

f1.inc()
f1.add()
f1.sub()
f1.mul()
f1.div()
f1.set(7, 11)
f2.inc()        // invalid, f2 is immutable and inc takes a mutable reference
f2.say_hello()  // invalid, say_hello does not take Foo as first argument

// scope operator can be used on the struct type to access the methods as regular functions
Foo::set(&var f1, 8, 12)
```

Values are automatically dereferenced for field access and referenced when calling methods.

```Self``` is a ```typealias``` for the struct type in methods. If the first parameter name is omitted in the function signature, then ```self``` is automatically inserted if the type is ```Self``` or a reference to it. Other than these two conveniences methods are exactly the same as regular functions. Neither ```Self``` or ```self``` are keywords.

## Sizeof

```rust
sizeof(i8)      // 1
sizeof(i64)     // 8
sizeof([i32:5]) // 4*5 = 20
```

Get the size of a type in bytes.

## Typeof

```rust
val a: i32
val b: typeof(a) // b has type i32

fun foo(c: i32, d: typeof(c)) {}

struct Bar {
    e: typeof(f)
    f: i32
}

```

```typeof``` takes an expression as an argument and can be used where a normal type can be used. The expression is only type checked and not evaluted anywhere else, i.e. it does not produce a value.

## Type Casting

```rust
val a: i64 = 5
val b = a as i32 // explicit cast
```

### Implicit Type Casting

```none
// integer casts are transitive, e.g. u8 to u64 or i64 is ok
i32 to i64
i16 to i32
i8 to i16

u32 to u64
u32 to i64
u16 to u32
u16 to i32
u8 to u16
u8 to i16

f32 to f64

// T generic type

&var T to &T
&var [T] to &[T]

T to &T
```

## If

```rust
val a = 5
if a > 5 {
    println("Big")
} elif a < 5 {
    println("Small")
} else {
    println("OK")
}
```

Braces required.

## For / While

```rust
for i = 0; i < 5; i++ {     // i immutable
    printiln(i)
}

for var i = 0; i < 5; i++ { // i mutable
    printiln(i)
    i++
}

var i = 0
while true {
    i++
    if (i % 2) == 0 {
        continue
    } elif i == 5 {
        break
    }
    printiln(i)
}
```

Braces required. ```continue``` and ```break``` as expected.

## Defer

```rust
{
    defer println("Bye")
    defer printiln(3)
    defer printiln(2)
    defer printiln(1)
    println("Hello")
}
// output:
// Hello
// 1
// 2
// 3
// Bye
```

Defer execution of a statement until the end of the block. If a defer is executed, the deferred statement is guaranteed to be executed at the end of the enclosing scope, regardless of the control flow. The deferred statements are executed in reverse order of the defers.

## Memory Management

Dynamic memory management is currently handled through the C API.

## C

Features to interface with the C ABI and runtime.

### Types

```none
c_void
c_char
c_uchar
c_short
c_ushort
c_int
c_uint
c_longlong
c_ulonglong
c_usize
c_float
c_double
```

References are currently used for C pointers, though no pointer arithmetic is allowed.

### Extern

```rust
// Functions defined in C can be called from Dingo
extern fun free(ptr: &c_void)
extern fun malloc(size: c_usize) &var c_void

// Functions defined in Dingo can be called from C
extern fun do_stuff() {
    //...
}
```

Using ```extern``` on functions will enable C ABI and disable name mangling.

### Main

```rust

extern fun main(argc: c_int, argv: &&c_uchar) c_int {
    return 0
}
```

Main function in Dingo.

## Strings

There are two types of string literals: normal and C-like. Normal string literals are immutable slices, and C-like strings are immutable references. Both string types are null-terminated.

```rust
val a: &[u8] = "Hello"
val b: &u8 = c"Bye
```

## Booleans

```rust
val a: bool = true
val b = false
```

## Numbers

### Types

```rust
// signed integers
i8
i16
i32
i64
// unsigned integers
u8
u16
u32
u64
usize
// floating point
f32
f64
```

### Literal Samples

```rust
15      // plain integer literal
100_000 // underscores to make large numbers more readable
100u8   // literals can have any numeric type as a suffix
0xFF    // hex
077     // octal
3.14    // plain floating point literal
1.234e2 // scientific notation: 1.234*10^2
1.234E2
```

## Basic Operators

### Binary Operators

```rust
a + b   // addition
a - b   // subtraction
a * b   // mutliplication
a / b   // division
a % b   // remainder

a >= b  // greater than or equal to
a <= b  // less than or equal to
a == b  // equality
a != b  // inequality

// lazily evaluted
a and b // logical and
a or b  // logical or
```

### Unary Operators

```rust
-a      // numerical negation
not a   // logical negation
```

## Assignments

```rust
a = 2   // normal assignment
a += 2  // a = a + 2
a -= 2  // a = a - 2
a *= 2  // a = a * 2
a /= 2  // a = a / 2
a %= 2  // a = a % 2
a++     // a = a + 1
a--     // a = a - 1
```

All assignments are statements.

## Operator Precedence

```none
Precedence  Associativity   Operation
1           None            (exp) len(a) sizeof(a) literal identifier
2           Left-to-right   a() a[] a[i] a[i:j] a.b
3           None            not -a &a
4           None            a as b
5           Left-to-right   a*b a/b a%b
6                           a+b a-b
7                           < <= > >=
8                           == !=
9                           and
10                          or
```

## Grammar

See [grammar](grammar.md).

## Naming Conventions

### Primitive Types

```none
snake_case
```

### User-Defined Types

```none
PascalCase
```

### Modules

```none
singleword
```

### Functions & Variables

```none
snake_case
```

## Keywords

```none
and
as
break
continue
defer
elif
else
extern
false
for
fun
if
import
include
len
module
not
null
or
priv
pub
return
sizeof
struct
true
typealias
typeof
use
val
var
while
```

## Dingo

Dingo is a statically typed and compiled programming language inspired by C++, Go and Rust. The focus is on easy C interop (from and to), full memory control, fast build times and expressivity.

Current planned features:
* Initial module system
* Function literals
* Methods and interfaces
* Templates
* Smart pointers

Note this project is still a work in progress.

## Example
```rust
module main

// External C functions
@c fun abs(_ i32) i32
@c fun putchar(_ i32) i32
@c fun puts(_ *i8) i32

/*
    Comment
    /*
        Nested comment
    */
*/
@c fun main() {
    // Infer type [5:i32]
    var arr = [31, 4, -10, 9, 2]

    bubbleSort(&var arr[:])
    puts(c"Sorted ints:")
    printArray(&arr[:])
}

fun swap(x *var i32, y *var i32) {
    val tmp = *x
    *x = *y
    *y = tmp
}

fun bubbleSort(arr *var [:i32]) {
    for i = 0; i < lenof(arr)-1; i++ {
        for j = 0; j < lenof(arr)-1; j++ {
            if arr[j] > arr[j+1] {
                swap(&var arr[j], &var arr[j+1])
            }
        }
    }
}

fun printArray(arr *[:i32]) {
    for i = 0; i < lenof(arr); i++ {
        puti(arr[i])
        puts(c"")
    }
}

// Recursively print each digit
fun puti(i i32) {
    val i2 = abs(i)
    if i2 < 10 {
        if i < 0 {
            putchar(45)
        }
    } else {
        puti(i/10)
    }
    putchar(48 + i2%10)
}
```

Output
```
Sorted ints:
-10
2
4
9
31
```

## Dependencies
* Go 
* LLVM 
* Go LLVM bindings

## Installation
TODO

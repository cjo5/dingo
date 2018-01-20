## Dingo

Dingo is a statically typed and compiled programming language focused on full memory control, fast build times, concise syntax, and easy interop from and to C. 

Note this project is still a work in progress.

## Example
```rust
module main

// C function declarations
fun[c] abs(i32) i32
fun[c] putchar(i32) i32
fun[c] puts(*i8) i32

/*
    Comment
    /*
        Nested comment
    */
*/
fun[c] main() {
    // Infer type [i32:5]
    var arr = [31, 4, -10, 9, 2]

    bubbleSort(&var arr[:])
    puts(c"Sorted ints:")
    printSlice(&arr[:])
}

fun swap(x *var i32, y *var i32) {
    val tmp = *x
    *x = *y
    *y = tmp
}

fun bubbleSort(slice *var [i32]) {
    for i = 0; i < lenof(slice)-1; i++ {
        for j = 0; j < lenof(slice)-1; j++ {
            if slice[j] > slice[j+1] {
                swap(&var slice[j], &var slice[j+1])
            }
        }
    }
}

fun printSlice(slice *[i32]) {
    for i = 0; i < lenof(slice); i++ {
        puti(slice[i])
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

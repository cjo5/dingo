## Dingo

Dingo is a statically typed and compiled programming language focused on concise syntax, fast build times, full memory control, and easy interop from and to C. 

## Dependencies
* Go >= 1.6.2
* LLVM 6.0 and Go bindings

## Example
```rust
// C function declarations
fun[c] abs(i32) i32
fun[c] putchar(i32) i32
fun[c] puts(&i8) i32

/*
    Comment
    /*
        Nested comment
    */
*/
fun[c] main() i32 {
    // Infer type [i32:5]
    var arr = [31, 4, -10, 9, 2]

    sort(&var arr[:])
    puts(c"Sorted ints:")
    printSlice(&arr[:])

    return 0
}

fun swap(x &var i32, y &var i32) {
    val tmp = *x
    *x = *y
    *y = tmp
}

fun sort(slice &var [i32]) {
    for i = 0; i < len(slice)-1; i++ {
        for j = 0; j < len(slice)-1; j++ {
            if slice[j] > slice[j+1] {
                swap(&var slice[j], &var slice[j+1])
            }
        }
    }
}

fun printSlice(slice &[i32]) {
    for i = 0; i < len(slice); i++ {
        puti(slice[i])
        puts(c"")
    }
}

// Recursively print each digit
fun puti(i i32) {
    val i2 = abs(i)
    if i2 < 10 {
        if i < 0 {
            putchar('-')
        }
    } else {
        puti(i/10)
    }
    putchar('0' + i2%10)
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

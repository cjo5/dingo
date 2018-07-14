## Dingo

Dingo is a statically typed and compiled programming language focused on concise syntax, fast build times, full memory control, and easy interop from and to C. 

## Dependencies
* Go >= 1.6.2
* LLVM 6.0 and Go bindings

## Example
```rust
// C function declarations
extern fun abs(c_int) c_int
extern fun putchar(c_int) c_int
extern fun puts(&c_char) c_int

/*
    Comment
    /*
        Nested comment
    */
*/
extern fun main() c_int {
    var arr = [i32]{31, 4, -10, 9, 2}

    sort(&var arr[:])
    puts(c"Sorted ints:")
    print_slice(&arr[:])

    return 0
}

fun swap(x &var i32, y &var i32) {
    val tmp = *x
    *x = *y
    *y = tmp
}

fun sort(slice &var [i32]) {
    for i usize = 0; i < len(slice)-1; i++ {
        for j usize = 0; j < len(slice)-1; j++ {
            if slice[j] > slice[j+1] {
                swap(&var slice[j], &var slice[j+1])
            }
        }
    }
}

fun print_slice(slice &[i32]) {
    for i usize = 0; i < len(slice); i++ {
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

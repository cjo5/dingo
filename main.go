package main

import (
	"fmt"

	"github.com/jhnl/interpreter/scanner"
	"github.com/jhnl/interpreter/token"
)

func main() {
	fmt.Println("Hello world")

	var s scanner.Scanner
	src := []byte(" x=5+2\n9")

	s.Init(src)

	for {
		lex := s.Scan()

		fmt.Println(lex)

		if lex.Tok == token.EOF {
			break
		}
	}
}

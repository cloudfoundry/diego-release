package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("I'm a custom entrypoint")
	fmt.Printf("I was called with: '%s'\n", os.Args[1:])
}

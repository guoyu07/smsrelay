package main

import (
	"fmt"
	// "testing"
)

func ExampleEncode() {
	v1 := []string{"c", "a", "b"}
	v2 := []string{"5", "4", "3"}

	fmt.Println(Encode(v1, v2))
	// Output:
	// c=5&a=4&b=3
}

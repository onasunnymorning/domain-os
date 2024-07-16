package main

import (
	"fmt"

	_ "github.com/lib/pq"
)

func main() {
	domain := "ex--ample.com"
	fmt.Println(domain[2:4])
}

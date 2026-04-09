package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("skill-go server starting...")
	fmt.Printf("pid: %d\n", os.Getpid())
	fmt.Println("skill-go server ready")
}

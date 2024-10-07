package main

import (
	"fmt"
	"github.com/unbasical/doras-server/pkg/doras"
)

func main() {
	msg := doras.Hello("test")
	fmt.Println(msg)
}

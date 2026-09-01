// Package main is the orchestrator entry point. It does nothing yet; it exists
// so that the toolchain is proven end to end before milestone α starts.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Printf("piton: toolchain ok (%s %s/%s)\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

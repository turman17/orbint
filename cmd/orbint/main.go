package main

import (
	"fmt"
	"os"
	"strings"
	"github.com/turman17/orbint/internal/orbit"
)

func main() {
	data, err := os.ReadFile("internal/orbit/testdata/iss.tle")
	check(err)

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")

	tle, err := orbit.ParseTle(lines)
	check(err)
	fmt.Printf("%+v\n", tle)
}

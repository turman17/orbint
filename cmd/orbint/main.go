package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/turman17/orbint/internal/orbit"
)

func main() {
	data, err := os.ReadFile("internal/orbit/testdata/iss.tle")
	if err != nil {
		fmt.Printf("Error reading TLE file: %s\n", err.Error())
		return
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	
	tle, err := orbit.ParseTle(lines)
	if err != nil {
		fmt.Printf("Error Parsing TLE: %s\n", err.Error())
	}
	fmt.Printf("%+v\n", tle)
}



package main

import (
	"strings"
)

func greet(name string) string {
	hello := "Hello, "
	name = strings.TrimSpace(name)

	if name == "" {
		return hello + "World!"
	}

	return hello + name + "!"
}

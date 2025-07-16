package main

import (
	"fmt"
	"strings"
)

func main() {
	fmt.Println(parseExtraParams("f:test"))
}

func parseExtraParams(keyword string) (bool, bool, string) {
	isFast := false
	translate := false
	if strings.HasPrefix(keyword, "f:") {
		keyword = keyword[2:]
		isFast = true
	}
	if strings.HasSuffix(keyword, ":t") {
		keyword = keyword[:len(keyword)-2]
		translate = true
	}
	return isFast, translate, keyword
}

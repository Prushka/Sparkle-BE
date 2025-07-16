package main

import (
	"Sparkle/ai"
	"fmt"
)

func main() {
	test := `-->
test

--->
te。st123。

--->

test123。
test123。
--->

test123
test123。
--->

test123test123。
--->

哈。

--->
哈。。

--->
哈。

--->
`
	fmt.Println(ai.TrimPeriods(test))
}

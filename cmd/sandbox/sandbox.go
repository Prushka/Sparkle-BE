package main

import (
	"fmt"
)

func main() {
	test := `WEBVTT


00:00:02.500 --> 00:00:04.300

00:00:02.500 --> 00:00:04.300
and the way we access it is changing。

00:00:02.500 --> 00:00:04.300
and the way we access it is changing。


00:00:02.500 --> 00:00:04.300
and the way we access it is changing。


00:00:02.500 --> 00:00:04.300
and the way we access it is changing。

00:00:02.500 --> 00:00:04.300
and the way we access it is changing!`
	fmt.Println(test)
}

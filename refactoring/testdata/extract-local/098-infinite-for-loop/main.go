package main

import "fmt"

func main() {
	a := 5
	b := 7
	for {
		c := a + b // <<<<< extractLocal,9,7,9,12,newVar,pass
		fmt.Println("c is", c)
		if c == 12 {
			break
		}
	}
}

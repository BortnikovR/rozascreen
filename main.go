package main

import (
	"github.com/bortnikovr/go/rozascreen/screener"
)

func main() {
	s, err := screener.NewScreener()
	if err != nil {
		panic(err)
	}
	s.Run()
}

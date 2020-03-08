package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/tiegz/pkgviz/pkg/pkgviz"
)

func main() {
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		log.Fatalln("error: no package name given")
		return
	}
	out := pkgviz.WriteGraph(args[0])
	fmt.Println(out)
}

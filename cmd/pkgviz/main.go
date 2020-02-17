package main

import (
	"flag"
	"fmt"
	"log"

	// command line:
	// go build pkgviz.go && ./pkgviz time | tee > foo.dot && dot -Tpng foo.dot -o foo.png && open foo.png
	pkgviz "github.com/tiegz/pkgviz/pkg/pkgviz"
)

func main() {
	flag.Parse()
	args := flag.Args()

	if len(args) < 1 {
		log.Fatalln("error: no package name given")
		return
	}

	out := pkgviz.WriteGraph(args[0])
	fmt.Println(out)
}

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/tiegz/pkgviz/pkg/pkgviz"
)

func main() {
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		log.Fatalln("error: no package name given")
		return
	}
	dotFile := pkgviz.WriteGraph(args[0])

	imageFilename := "out.png"
	cmd := exec.Command("dot", "-Tpng", "-o", imageFilename)
	stdin, _ := cmd.StdinPipe()
	go func() {
		defer stdin.Close()
		io.WriteString(stdin, dotFile)
	}()

	if listCmdOut, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Error running '%v'\n", cmd.String())
		fmt.Printf("Debug: %s\n", string(listCmdOut))
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Println(imageFilename)
}

# pkgviz-go

[![Go Report Card](https://goreportcard.com/badge/github.com/tiegz/pkgviz-go)](https://goreportcard.com/report/github.com/tiegz/pkgviz-go)
[![GoDoc](https://godoc.org/github.comtiegz/pkgviz-go?status.svg)](https://godoc.org/github.com/tiegz/pkgviz-go)

Generate a vizualization of a Go package's types.

## How does it work

`pkgviz-go` uses Go's [type-checker](https://godoc.org/go/types) to analyse a given go package, builds a graph of the types, writes it to [DOT format](https://en.wikipedia.org/wiki/DOT_%28graph_description_language%29), and generates an image of the graph using [graphviz](https://graphviz.org/).

## Installation

Ensure that you have [graphviz](https://www.graphviz.org/) installed:

* MacOS: `brew install graphviz`
* Windows: install the latest package from [here](https://graphviz.gitlab.io/_pages/Download/Download_windows.html)
* Linux: follow your distribution's instructions [here](https://graphviz.gitlab.io/download/)

Then install the `pkgviz` command:

`go install github.com/tiegz/pkgviz-go/cmd/pkgviz`

## Usage

`pkgviz A_GO_PKGNAME`

The graph image is output to `out.png`.

### Examples:

`pkgviz github.com/tiegz/pkgviz-go`

<img width="500px" src="https://github.com/tiegz/pkgviz-go/raw/master/out-pkgviz-go.png">

`pkgviz time`

<img width="500px" src="https://github.com/tiegz/pkgviz-go/raw/master/out-time.png">


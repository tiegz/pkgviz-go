# pkgviz-go

Visualize the relationships of types in a Go package.

## How does it work

`pkgviz-go` uses Go's [type-checker](https://godoc.org/go/types) to analyse a given go package, builds a graph of the types, writes it to [DOT format](https://en.wikipedia.org/wiki/DOT_%28graph_description_language%29), and generates an image of the graph using [graphviz](https://graphviz.org/).

## Installation

Ensure that you have [graphviz](https://www.graphviz.org/) installed.

MacOS: `brew install graphviz`

Then install the `pkgviz` command:

`go install github.com/tiegz/pkgviz-go`

## Usage

`pkgviz A_GO_PKGNAME`

### Examples:

`pkgviz github.com/tiegz/pkgviz-go`
`pkgviz time`

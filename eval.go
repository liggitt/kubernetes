package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

func isStandard(p string) bool {
	segments := strings.Split(p, "/")
	return !strings.Contains(segments[0], ".")
}

func main() {

	ps, err := packages.Load(&packages.Config{
		Mode:  packages.LoadImports,
		Env:   append(os.Environ(), "GO111MODULE=off", "GOOS=linux", "GOARCH=amd64"),
		Tests: true,
	}, "./...")
	fmt.Println(err)
	packages.Visit(ps, func(p *packages.Package) bool {
		return true
	}, func(p *packages.Package) {
		if strings.HasPrefix(p.PkgPath, "k8s.io/") && !strings.Contains(p.PkgPath, "vendor") {
			return
		}
		if isStandard(p.PkgPath) {
			return
		}
		fmt.Println(p.PkgPath, len(p.Imports))
	})
	return

	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: go run eval.go deps.json")
		fmt.Fprintln(os.Stderr, "Where deps.json is the output of `go list -mod=vendor -json -tags linux -e all`")
		os.Exit(1)
	}

	inputFile := os.Args[1]

	f, err := os.Open(inputFile)
	checkErr(err)
	defer f.Close()

	decoder := json.NewDecoder(f)
	packages := []Package{}
	for {
		p := &Package{}
		err := decoder.Decode(p)
		if err == nil {
			packages = append(packages, *p)
			continue
		}
		if err == io.EOF {
			break
		}
		checkErr(err)
	}

	standard := map[string]bool{}
	incoming := map[string]map[string]bool{}
	outgoing := map[string][]string{}
	for _, p := range packages {
		if p.Standard || p.ImportPath == "runtime/cgo" {
			standard[p.ImportPath] = true
		}
	}
	for _, p := range packages {
		if p.Standard || p.ImportPath == "runtime/cgo" {
			continue
		}
		if strings.HasPrefix(p.ImportPath, "k8s.io/") {
			continue
		}

		outgoing[p.ImportPath] = p.Deps
		for _, dep := range p.Deps {
			if standard[dep] || dep == "runtime/cgo" {
				continue
			}
			if len(incoming[dep]) == 0 {
				incoming[dep] = map[string]bool{}
			}
			incoming[dep][p.ImportPath] = true
		}
	}

	incomingSorted := make([]string, 0, len(incoming))
	for p, _ := range incoming {
		incomingSorted = append(incomingSorted, p)
	}
	sort.Strings(incomingSorted)
	sort.SliceStable(incomingSorted, func(i, j int) bool { return len(incoming[incomingSorted[i]]) > len(incoming[incomingSorted[j]]) })

	outgoingSorted := make([]string, 0, len(outgoing))
	for p, _ := range outgoing {
		outgoingSorted = append(outgoingSorted, p)
	}
	sort.Strings(outgoingSorted)
	sort.SliceStable(outgoingSorted, func(i, j int) bool { return len(outgoing[outgoingSorted[i]]) > len(outgoing[outgoingSorted[j]]) })

	fmt.Println("outgoing:")
	for _, p := range outgoingSorted[:30] {
		fmt.Printf("%d %s\n", len(outgoing[p]), p)
	}

	fmt.Println("incoming:")
	for _, p := range incomingSorted[:30] {
		fmt.Printf("%d %s\n", len(incoming[p]), p)
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type Package struct {
	Dir        string
	ImportPath string
	Standard   bool
	Deps       []string
}

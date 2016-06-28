package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"text/scanner"

	"github.com/NetSys/quilt/stitch"
)

const quiltPath = "QUILT_PATH"

type argOption struct {
	result interface{} // Optionally store result.
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: inspect <path to spec file> [commands]\n"+
		"Options\n"+
		" - viz <pdf|ascii>\n"+
		" - check <path to invariants file>\n"+
		" - query [must have check] <path to query file>")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("not enough arguments: ", len(os.Args))
		usage()
	}

	configPath := os.Args[1]

	f, err := os.Open(configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	sc := scanner.Scanner{
		Position: scanner.Position{
			Filename: configPath,
		},
	}
	pathStr, _ := os.LookupEnv(quiltPath)
	pathSlice := strings.Split(pathStr, ":")
	spec, err := stitch.New(*sc.Init(bufio.NewReader(f)), pathSlice)
	if err != nil {
		panic(err)
	}

	containerLabels := make(map[string][]*stitch.Container)
	for _, container := range spec.QueryContainers() {
		labels := container.Labels()
		for _, label := range labels {
			if _, have := containerLabels[label]; !have {
				containerLabels[label] = []*stitch.Container{}
			}
			containerLabels[label] = append(containerLabels[label], container)
		}
	}

	graph := makeGraph()
	for _, conn := range spec.QueryConnections() {
		graph.addConnection(conn.From, conn.To)
	}

	for _, pl := range spec.QueryPlacements() {
		graph.addPlacementRule(pl)
	}

	ignoreNext := 0
	foundFlags := map[string]argOption{}
	func() {
		args := os.Args[2:]
		for i, arg := range args {
			switch {
			case ignoreNext > 0:
				ignoreNext--
			case arg == "viz":
				foundFlags[arg] = argOption{}
				outputFormat := args[i+1] // 'pdf' or 'ascii'
				viz(configPath, spec, graph, outputFormat)
				ignoreNext = 1
			case arg == "check":
				invs, failer, err := check(graph, args[i+1])
				if err != nil && failer == nil {
					fmt.Printf("parsing invariants failed: %s", err)
				} else if err != nil {
					fmt.Println("invariant failed: ", failer.str)
				} else {
					fmt.Println("invariants passed")
				}
				foundFlags[arg] = argOption{result: invs}
				ignoreNext = 1
			case arg == "query":
				foundFlags[arg] = argOption{}

				defer func(i int) {
					if checkOpt, ok := foundFlags["check"]; !ok {
						fmt.Println("query without check")
						usage()
					} else {
						invs := checkOpt.result.([]invariant)
						_, _, err := ask(graph, invs, args[i+1])
						if err != nil {
							fmt.Println(err)
						} else {
							fmt.Println("query passed invariants")
						}
					}
				}(i)
				ignoreNext = 1
			default:
				fmt.Println("unknown arg", arg)
				usage()
			}
		}
	}()
}

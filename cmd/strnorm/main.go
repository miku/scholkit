package main

import (
	"flag"
	"log"
	"os"

	"github.com/miku/scholkit/normal"
	"github.com/miku/scholkit/parallel"
)

var (
	algo = flag.String("a", "simple", "normalization algorithm")
)

func main() {
	flag.Parse()
	var normalizer normal.Normalizer
	switch {
	case *algo == "simple": // lowercase
		normalizer = &normal.SimpleNormalizer{}
	case *algo == "nows": // no whitespace
		normalizer = &normal.RemoveWSNormalizer{}
	case *algo == "lo": // letter only
		normalizer = &normal.LettersOnlyNormalizer{}
	case *algo == "nowslo": // no ws, letters only
		normalizer = &normal.Pipeline{Normalizer: []normal.Normalizer{
			&normal.SimpleNormalizer{},
			&normal.RemoveWSNormalizer{},
			&normal.LettersOnlyNormalizer{},
		}}
	default:
		log.Fatalf("invalid normalizer name: %s", *algo)
	}
	pp := parallel.NewProcessor(os.Stdin, os.Stdout, normal.ProcNormAdapt(normalizer.Normalize))
	if err := pp.Run(); err != nil {
		log.Fatal(err)
	}
}

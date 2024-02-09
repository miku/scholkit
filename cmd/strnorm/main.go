package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"unicode"

	"github.com/miku/scholkit/parallel"
)

var (
	algo = flag.String("a", "simple", "normalization algorithm")
)

type Pipeline struct {
	Normalizer []Normalizer
}

func (p *Pipeline) Normalize(s string) string {
	for _, n := range p.Normalizer {
		s = n.Normalize(s)
	}
	return s
}

type Normalizer interface {
	Normalize(string) string
}

type SimpleNormalizer struct{}

func (s *SimpleNormalizer) Normalize(v string) string {
	return strings.ToLower(v)
}

type RemoveWSNormalizer struct{}

func (s *RemoveWSNormalizer) Normalize(v string) string {
	var b strings.Builder
	for _, c := range v {
		if unicode.IsSpace(c) {
			continue
		}
		b.WriteRune(c)
	}
	b.WriteRune('\n') // re-add final newline
	return b.String()
}

type LettersOnlyNormalizer struct{}

func (s *LettersOnlyNormalizer) Normalize(v string) string {
	var b strings.Builder
	for _, c := range v {
		if !unicode.IsLetter(c) {
			continue
		}
		b.WriteRune(c)
	}
	b.WriteRune('\n') // re-add final newline
	return b.String()
}

func ProcNormAdapt(f func(string) string) func(b []byte) ([]byte, error) {
	return func(b []byte) ([]byte, error) {
		s := f(string(b))
		return []byte(s), nil
	}
}

func main() {
	flag.Parse()
	var normalizer Normalizer
	switch {
	case *algo == "simple":
		normalizer = &SimpleNormalizer{}
	case *algo == "nows":
		normalizer = &RemoveWSNormalizer{}
	case *algo == "lo":
		normalizer = &LettersOnlyNormalizer{}
	case *algo == "nowslo": // no ws, letters only
		normalizer = &Pipeline{Normalizer: []Normalizer{
			&SimpleNormalizer{},
			&RemoveWSNormalizer{},
			&LettersOnlyNormalizer{},
		}}
	default:
		log.Fatalf("invalid normalizer name: %s", *algo)
	}
	pp := parallel.NewProcessor(os.Stdin, os.Stdout, ProcNormAdapt(normalizer.Normalize))
	if err := pp.Run(); err != nil {
		log.Fatal(err)
	}
}

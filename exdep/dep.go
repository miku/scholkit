package exdep

import (
	"fmt"
	"os/exec"
	"strings"
)

// Dep represents an external tool dependency
type Dep struct {
	Name  string
	Links []string
	Docs  string
}

func Check(deps []Dep) []error {
	var errors []error
	for _, dep := range deps {
		if err := check(dep); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

func check(dep Dep) error {
	_, err := exec.LookPath(dep.Name)
	if err != nil {
		return fmt.Errorf("%s: %w [%s, %s]",
			dep.Name, err, dep.Docs, strings.Join(dep.Links, ", "))
	}
	return nil
}

package exdep

import (
	"fmt"
	"os/exec"
)

// Dep represents an external tool dependency
type Dep struct {
	Name string
	Docs string
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
		return fmt.Errorf("dependency '%s' not found: %w [%s]",
			dep.Name, err, dep.Docs)
	}
	return nil
}

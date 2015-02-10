package observer

import (
	"fmt"
	"os"
)

type ModuleInfo struct {
	InputQueue string
	RunnerFunc func() interface{}
}

// AvailableModules stores a list of activated module with their runner
var AvailableModules = make(map[string]ModuleInfo)

// RegisterModule adds a module to the list of available modules
func RegisterModule(name string, info ModuleInfo) {
	if _, exist := AvailableModules[name]; exist {
		fmt.Fprintf(os.Stderr, "RegisterModule: a module named '%s' has already been registered.\nAre you trying to import the same module twice?\n", name)
		os.Exit(1)
	}
	AvailableModules[name] = info
}

// Moduler provides the interface to a Module
type Moduler interface {
	Run([]byte) string
}

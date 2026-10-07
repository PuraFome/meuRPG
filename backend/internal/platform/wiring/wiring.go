// Package wiring is the check that cmd/api's wiring is complete.
//
// campaigns, characters, play, maps and progression need each other, so a few
// of their collaborators are connected after construction by a Set... call
// (cmd/api, in a fixed order). The compiler cannot see a call that is missing,
// and a nil collaborator rarely fails loudly: a nil fog source means "no fog of
// war", and players would see the NPCs the master hid (RN-10), with no error
// anywhere. Each module's CheckWired lists what it needs, and cmd/api calls it
// at the end of the wiring: a missing call stops the server at startup.
package wiring

import (
	"fmt"
	"strings"
)

// Dep is one collaborator connected by a setter, and whether it is still nil.
type Dep struct {
	Setter  string
	Missing bool
}

// Check returns an error naming every setter of module that was never called,
// or nil when all were.
func Check(module string, deps ...Dep) error {
	var names []string
	for _, d := range deps {
		if d.Missing {
			names = append(names, d.Setter)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return fmt.Errorf("%s is not fully wired: %s never called", module, strings.Join(names, ", "))
}

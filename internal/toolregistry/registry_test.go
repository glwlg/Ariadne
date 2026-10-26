package toolregistry

import (
	"strings"
	"testing"
)

func TestDefinitionsHaveUniqueCommandsAndValidGeometry(t *testing.T) {
	commands := map[string]bool{}
	if len(definitions) != 10 {
		t.Fatalf("unexpected tool count: %d", len(definitions))
	}
	for view, definition := range definitions {
		if !strings.HasPrefix(definition.Title, "Ariadne - ") || definition.Command == "" || commands[definition.Command] {
			t.Fatalf("invalid tool command or title for %s: %#v", view, definition)
		}
		commands[definition.Command] = true
		if definition.MinWidth <= 0 || definition.MinHeight <= 0 || definition.Width < definition.MinWidth || definition.Height < definition.MinHeight {
			t.Fatalf("invalid geometry for %s: %#v", view, definition)
		}
	}
	if _, ok := Lookup("launcher"); ok {
		t.Fatal("launcher must retain its own window lifecycle")
	}
}

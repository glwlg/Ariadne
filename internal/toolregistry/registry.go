package toolregistry

import (
	_ "embed"
	"encoding/json"
)

type Definition struct {
	Title     string `json:"title"`
	Command   string `json:"command"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	MinWidth  int    `json:"minWidth"`
	MinHeight int    `json:"minHeight"`
}

//go:embed views.json
var definitionsJSON []byte

var definitions = func() map[string]Definition {
	var views map[string]Definition
	if err := json.Unmarshal(definitionsJSON, &views); err != nil {
		panic(err)
	}
	return views
}()

func Lookup(view string) (Definition, bool) {
	definition, ok := definitions[view]
	return definition, ok
}

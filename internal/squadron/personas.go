package squadron

import "fmt"

// Persona is an optional character voice applied to a squadron agent.
// The Preamble is prepended to the agent's full prompt, above the system
// prompt, shaping the agent's voice in work and in channel messages.
type Persona struct {
	Name        string // short key, e.g. "overconfident-engineer"
	DisplayName string // human label, e.g. "Overconfident Engineer"
	Preamble    string // text injected at the top of the agent's prompt
}

// personas holds the built-in persona library. Populated in init() from the
// persona_defs block below so the map literal stays readable.
var personas = map[string]Persona{}

// LookupPersona returns the built-in persona with the given key. The second
// return value is false if no such persona exists.
func LookupPersona(name string) (Persona, bool) {
	p, ok := personas[name]
	return p, ok
}

// ApplyPersona prepends the persona preamble above the given prompt.
func ApplyPersona(p Persona, prompt string) string {
	return fmt.Sprintf("%s\n\n---\n\n%s", p.Preamble, prompt)
}

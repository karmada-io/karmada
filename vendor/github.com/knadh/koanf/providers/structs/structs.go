// Package structs implements a koanf.Provider that takes a struct and tag
// and returns a nested config map (using fatih/structs) to provide it to koanf.
package structs

import (
	"errors"

	"github.com/fatih/structs"

	"github.com/knadh/koanf/maps"
)

// Structs implements a structs provider.
type Structs struct {
	s     interface{}
	tag   string
	delim string
}

// Provider returns a provider that takes a takes a struct and a struct tag
// and uses structs to parse and provide it to koanf.
func Provider(s interface{}, tag string) *Structs {
	return &Structs{s: s, tag: tag}
}

// ProviderWithDelim returns a provider that takes a struct and a struct tag
// along with a delim and uses structs to parse and provide it to koanf.
func ProviderWithDelim(s interface{}, tag, delim string) *Structs {
	return &Structs{s: s, tag: tag, delim: delim}
}

// ReadBytes is not supported by the structs provider.
func (s *Structs) ReadBytes() ([]byte, error) {
	return nil, errors.New("structs provider does not support this method")
}

// Read reads the struct and returns a nested config map.
func (s *Structs) Read() (map[string]interface{}, error) {
	ns := structs.New(s.s)
	ns.TagName = s.tag

	out := ns.Map()

	if s.delim != "" {
		out = maps.Unflatten(out, s.delim)
	}

	return out, nil
}

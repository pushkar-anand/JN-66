package parser

import (
	"fmt"
	"strings"
)

// Registry holds all registered parsers and selects one based on header auto-detection.
// New format versions add a new parser — old ones are never removed, keeping backwards compatibility.
type Registry struct {
	parsers []Parser
}

// NewRegistry returns a Registry pre-loaded with all known bank parsers.
func NewRegistry() *Registry {
	r := &Registry{}
	r.Register(&AxisV1{})
	r.Register(&IDFCV1{})
	r.Register(&SBIV1{})
	r.Register(&ICICIV1{})
	return r
}

// Register adds a parser to the registry. Later registrations take priority in Detect.
func (r *Registry) Register(p Parser) {
	r.parsers = append([]Parser{p}, r.parsers...)
}

// ByBank returns the most recently registered parser for the named bank, or an error.
func (r *Registry) ByBank(bank string) (Parser, error) {
	bank = strings.ToLower(bank)
	for _, p := range r.parsers {
		if p.Bank() == bank {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unknown bank %q — supported: axis, idfc, sbi, icici", bank)
}

// Detect returns the first parser whose CanParse returns true for the given header row.
func (r *Registry) Detect(header []string) (Parser, error) {
	norm := make([]string, len(header))
	for i, h := range header {
		norm[i] = strings.ToLower(strings.TrimSpace(h))
	}
	for _, p := range r.parsers {
		if p.CanParse(norm) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no parser matched header: %v", header)
}

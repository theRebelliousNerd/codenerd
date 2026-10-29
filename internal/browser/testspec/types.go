// Package testspec defines bounded, portable declarative browser fixtures.
//
// The workflow is adapted from BrowserNERD under Apache-2.0. Fixtures reuse
// codeNERD's closed browser action vocabulary, selector-free semantic
// matchers, and read-only live-kernel assertions.
package testspec

import "codenerd/internal/browser"

const (
	MaxFixtureBytes = 256 << 10
	MaxActions      = 25
	MaxAssertions   = 100
	MaxQueryBytes   = 512
	MaxNameBytes    = 200
)

// Assertion is one present/absent browser fact check.
type Assertion struct {
	Name   string `json:"name" yaml:"name"`
	Query  string `json:"query" yaml:"query"`
	Expect string `json:"expect,omitempty" yaml:"expect,omitempty"`
	Scope  string `json:"scope,omitempty" yaml:"scope,omitempty"`
}

// Spec is one portable browser action and assertion fixture.
type Spec struct {
	Name       string                    `json:"name" yaml:"name"`
	SessionID  string                    `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	Actions    []browser.ActionOperation `json:"actions,omitempty" yaml:"actions,omitempty"`
	Assertions []Assertion               `json:"assertions" yaml:"assertions"`
}

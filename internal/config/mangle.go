package config

// MangleConfig configures the Mangle kernel.
type MangleConfig struct {
	SchemaPath       string `yaml:"schema_path"`
	PolicyPath       string `yaml:"policy_path"`
	FactLimit        int    `yaml:"fact_limit"`
	DerivedFactLimit int    `yaml:"derived_fact_limit"` // Max derived facts during evaluation (default: 500000)
	QueryTimeout     string `yaml:"query_timeout"`
}

// DefaultDerivedFactLimit is the default maximum derived facts during evaluation.
const DefaultDerivedFactLimit = 500000

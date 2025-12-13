package config

// MangleConfig configures the Mangle kernel.
type MangleConfig struct {
	SchemaPath   string `yaml:"schema_path"`
	PolicyPath   string `yaml:"policy_path"`
	FactLimit    int    `yaml:"fact_limit"`
	QueryTimeout string `yaml:"query_timeout"`
}

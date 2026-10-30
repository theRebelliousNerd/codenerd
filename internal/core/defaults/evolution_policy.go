package defaults

import _ "embed"

// EvolutionSchema and EvolutionPolicy are shared by the action kernel and the
// isolated adaptation kernel. The latter owns one group's temporary facts so
// background work never mutates an interactive turn's inference state.
//
//go:embed schemas_evolution.mg
var EvolutionSchema string

//go:embed policy/prompt_evolution.mg
var EvolutionPolicy string

// ConfigParamsPolicy is the config_param bridge (Decls and the missing-knob
// rule) that a component kernel loads beside its own policy.
//
//go:embed policy/config_params.mg
var ConfigParamsPolicy string

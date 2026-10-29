package config

import "fmt"

// WorkMode is how far codeNERD acts on its own. One switch (Shift+Tab in the
// chat, /yolo, --yolo for a session) moves through all three:
//
//   - yolo: codeNERD judges when work is ready and starts it, makes every
//     choice it can alone, and approves its own human-approval prompts.
//     It resolves CHOICE points, never SAFETY verdicts: constitutional
//     denials, the Dreamer's unliftable warnings and safety validators still
//     refuse, and unbounded /recurse still stops on its stall fuse.
//   - propose: codeNERD proposes the work and starts it on your yes; a
//     multi-step run pauses before each mutation.
//   - explicit: work starts only on your go; a multi-step run pauses after
//     every step.
type WorkMode string

const (
	WorkModeYolo     WorkMode = "yolo"
	WorkModePropose  WorkMode = "propose"
	WorkModeExplicit WorkMode = "explicit"
)

// WorkModes is the Shift+Tab cycle order.
var WorkModes = []WorkMode{WorkModeYolo, WorkModePropose, WorkModeExplicit}

// Valid reports whether m is one of the three modes. Empty is not valid here;
// EffectiveWorkMode reads an absent key as propose.
func (m WorkMode) Valid() bool {
	for _, w := range WorkModes {
		if m == w {
			return true
		}
	}
	return false
}

// Next is the mode after m in the Shift+Tab cycle.
func (m WorkMode) Next() WorkMode {
	for i, w := range WorkModes {
		if m == w {
			return WorkModes[(i+1)%len(WorkModes)]
		}
	}
	return WorkModePropose
}

// EffectiveWorkMode is the configured mode, propose when unset. Nil-safe.
func (c *UserConfig) EffectiveWorkMode() WorkMode {
	if c == nil || c.WorkMode == "" {
		return WorkModePropose
	}
	return c.WorkMode
}

// YoloMode reports whether the persisted mode is yolo. Nil-safe: no config
// means no yolo.
func (c *UserConfig) YoloMode() bool {
	return c.EffectiveWorkMode() == WorkModeYolo
}

func checkWorkMode(m WorkMode) error {
	if m == "" || m.Valid() {
		return nil
	}
	return fmt.Errorf("work_mode %q is not one of yolo, propose, explicit", m)
}

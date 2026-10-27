package config

// BrowserPolicyConfig is the browser policy's numeric threshold and action
// ranking section. Pointer fields preserve the distinction between a missing
// setting (which takes the pinned default) and an explicit zero (which Check
// can reject where zero would disable a required threshold or priority).
type BrowserPolicyConfig struct {
	SlowAPIMs                  *int `json:"slow_api_ms,omitempty"`
	CausalBucketMs             *int `json:"causal_bucket_ms,omitempty"`
	UnhydratedAgeMs            *int `json:"unhydrated_age_ms,omitempty"`
	AttendedNavigationWindowMs *int `json:"attended_navigation_window_ms,omitempty"`
	PriorityPrimary            *int `json:"priority_primary,omitempty"`
	PriorityButton             *int `json:"priority_button,omitempty"`
	PriorityInput              *int `json:"priority_input,omitempty"`
	PrioritySelect             *int `json:"priority_select,omitempty"`
	PriorityCheckbox           *int `json:"priority_checkbox,omitempty"`
	PriorityRadio              *int `json:"priority_radio,omitempty"`
	PriorityLink               *int `json:"priority_link,omitempty"`
	PriorityTab                *int `json:"priority_tab,omitempty"`
	PriorityComboboxClick      *int `json:"priority_combobox_click,omitempty"`
	PriorityComboboxType       *int `json:"priority_combobox_type,omitempty"`
	PriorityMenuitem           *int `json:"priority_menuitem,omitempty"`
	PriorityOption             *int `json:"priority_option,omitempty"`
	PriorityClickable          *int `json:"priority_clickable,omitempty"`
}

// DefaultBrowserPolicyConfig records the existing browser policy fixture
// values as config defaults. Ranking remains policy data: these values mirror
// the production browser policy test corpus rather than choosing new Go-side
// behavior.
func DefaultBrowserPolicyConfig() BrowserPolicyConfig {
	return BrowserPolicyConfig{
		SlowAPIMs:                  browserIntPointer(1000),
		CausalBucketMs:             browserIntPointer(2000),
		UnhydratedAgeMs:            browserIntPointer(3000),
		AttendedNavigationWindowMs: browserIntPointer(0),
		PriorityPrimary:            browserIntPointer(100),
		PriorityButton:             browserIntPointer(80),
		PriorityInput:              browserIntPointer(78),
		PrioritySelect:             browserIntPointer(72),
		PriorityCheckbox:           browserIntPointer(68),
		PriorityRadio:              browserIntPointer(66),
		PriorityLink:               browserIntPointer(60),
		PriorityTab:                browserIntPointer(64),
		PriorityComboboxClick:      browserIntPointer(62),
		PriorityComboboxType:       browserIntPointer(76),
		PriorityMenuitem:           browserIntPointer(60),
		PriorityOption:             browserIntPointer(58),
		PriorityClickable:          browserIntPointer(56),
	}
}

func browserIntPointer(value int) *int { return &value }

// WithDefaults fills only omitted policy settings. It copies every pointer so
// a normalized config can be changed without mutating its source.
func (c BrowserPolicyConfig) WithDefaults() BrowserPolicyConfig {
	d := DefaultBrowserPolicyConfig()
	fill := func(value, fallback *int) *int {
		if value == nil {
			value = fallback
		}
		return browserIntPointer(*value)
	}
	c.SlowAPIMs = fill(c.SlowAPIMs, d.SlowAPIMs)
	c.CausalBucketMs = fill(c.CausalBucketMs, d.CausalBucketMs)
	c.UnhydratedAgeMs = fill(c.UnhydratedAgeMs, d.UnhydratedAgeMs)
	c.AttendedNavigationWindowMs = fill(c.AttendedNavigationWindowMs, d.AttendedNavigationWindowMs)
	c.PriorityPrimary = fill(c.PriorityPrimary, d.PriorityPrimary)
	c.PriorityButton = fill(c.PriorityButton, d.PriorityButton)
	c.PriorityInput = fill(c.PriorityInput, d.PriorityInput)
	c.PrioritySelect = fill(c.PrioritySelect, d.PrioritySelect)
	c.PriorityCheckbox = fill(c.PriorityCheckbox, d.PriorityCheckbox)
	c.PriorityRadio = fill(c.PriorityRadio, d.PriorityRadio)
	c.PriorityLink = fill(c.PriorityLink, d.PriorityLink)
	c.PriorityTab = fill(c.PriorityTab, d.PriorityTab)
	c.PriorityComboboxClick = fill(c.PriorityComboboxClick, d.PriorityComboboxClick)
	c.PriorityComboboxType = fill(c.PriorityComboboxType, d.PriorityComboboxType)
	c.PriorityMenuitem = fill(c.PriorityMenuitem, d.PriorityMenuitem)
	c.PriorityOption = fill(c.PriorityOption, d.PriorityOption)
	c.PriorityClickable = fill(c.PriorityClickable, d.PriorityClickable)
	return c
}

// Check reports explicit browser policy values that would make a required
// threshold or rank unusable. A zero attended-navigation window is meaningful
// and matches the existing policy fixture; negative windows are rejected.
func (c BrowserPolicyConfig) Check(prefix string) []Problem {
	c = c.WithDefaults()
	var out []Problem
	positive := []struct {
		name  string
		value *int
	}{
		{"slow_api_ms", c.SlowAPIMs},
		{"causal_bucket_ms", c.CausalBucketMs},
		{"unhydrated_age_ms", c.UnhydratedAgeMs},
		{"priority_primary", c.PriorityPrimary},
		{"priority_button", c.PriorityButton},
		{"priority_input", c.PriorityInput},
		{"priority_select", c.PrioritySelect},
		{"priority_checkbox", c.PriorityCheckbox},
		{"priority_radio", c.PriorityRadio},
		{"priority_link", c.PriorityLink},
		{"priority_tab", c.PriorityTab},
		{"priority_combobox_click", c.PriorityComboboxClick},
		{"priority_combobox_type", c.PriorityComboboxType},
		{"priority_menuitem", c.PriorityMenuitem},
		{"priority_option", c.PriorityOption},
		{"priority_clickable", c.PriorityClickable},
	}
	for _, field := range positive {
		if *field.value <= 0 {
			out = append(out, Problem{
				Severity: SeverityError,
				Path:     prefix + "." + field.name,
				Message:  "must be a positive integer",
				Fix:      "set a value greater than zero, or remove the key for the default",
			})
		}
	}
	if *c.AttendedNavigationWindowMs < 0 {
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     prefix + ".attended_navigation_window_ms",
			Message:  "must be zero or a positive integer",
			Fix:      "set zero or a positive duration in milliseconds, or remove the key for the default",
		})
	}
	return out
}

// Params returns every threshold and rank read by policy/browser.mg as the
// generic config_param facts required by the browser producer.
func (c BrowserPolicyConfig) Params() []Param {
	c = c.WithDefaults()
	return []Param{
		{Key: "/browser_slow_api_ms", Value: int64(*c.SlowAPIMs)},
		{Key: "/browser_causal_bucket_ms", Value: int64(*c.CausalBucketMs)},
		{Key: "/browser_unhydrated_age_ms", Value: int64(*c.UnhydratedAgeMs)},
		{Key: "/browser_attended_navigation_window_ms", Value: int64(*c.AttendedNavigationWindowMs)},
		{Key: "/browser_priority_primary", Value: int64(*c.PriorityPrimary)},
		{Key: "/browser_priority_button", Value: int64(*c.PriorityButton)},
		{Key: "/browser_priority_input", Value: int64(*c.PriorityInput)},
		{Key: "/browser_priority_select", Value: int64(*c.PrioritySelect)},
		{Key: "/browser_priority_checkbox", Value: int64(*c.PriorityCheckbox)},
		{Key: "/browser_priority_radio", Value: int64(*c.PriorityRadio)},
		{Key: "/browser_priority_link", Value: int64(*c.PriorityLink)},
		{Key: "/browser_priority_tab", Value: int64(*c.PriorityTab)},
		{Key: "/browser_priority_combobox_click", Value: int64(*c.PriorityComboboxClick)},
		{Key: "/browser_priority_combobox_type", Value: int64(*c.PriorityComboboxType)},
		{Key: "/browser_priority_menuitem", Value: int64(*c.PriorityMenuitem)},
		{Key: "/browser_priority_option", Value: int64(*c.PriorityOption)},
		{Key: "/browser_priority_clickable", Value: int64(*c.PriorityClickable)},
	}
}

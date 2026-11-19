package engine

// RuleEngineVersion identifies expression semantics independently of the legacy engine.
const RuleEngineVersion = "2.0.0"

type RuleSet struct {
	Version    int                   `json:"version"`
	Groups     []GroupDefinition     `json:"groups"`
	Components []ComponentDefinition `json:"components"`
	Inputs     []InputDefinition     `json:"inputs"`
}

type Expression struct {
	Op    string       `json:"op"`
	Value string       `json:"value,omitempty"`
	Ref   string       `json:"ref,omitempty"`
	Args  []Expression `json:"args,omitempty"`
}

type GroupDefinition struct {
	Code     string      `json:"code"`
	Name     string      `json:"name"`
	InGross  bool        `json:"inGross"`
	Target   *Expression `json:"target,omitempty"`
	Balancer string      `json:"balancer,omitempty"`
}

type ComponentDefinition struct {
	Code  string            `json:"code"`
	Name  string            `json:"name"`
	Group string            `json:"group"`
	Rules []CalculationRule `json:"rules"`
}

type CalculationRule struct {
	ID             string      `json:"id,omitempty"`
	When           *Expression `json:"when,omitempty"`
	Eligibility    *Expression `json:"eligibility,omitempty"`
	Expression     *Expression `json:"expression,omitempty"`
	Period         string      `json:"period"`
	Status         string      `json:"status,omitempty"`
	Min            *Expression `json:"min,omitempty"`
	Max            *Expression `json:"max,omitempty"`
	LimitsTogether bool        `json:"limitsTogether,omitempty"`
	Notes          string      `json:"notes,omitempty"`
}

type InputDefinition struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}

type GroupResult struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	InGross    bool    `json:"inGross"`
	Target     *string `json:"target,omitempty"`
	Monthly    *string `json:"monthly"`
	Annual     *string `json:"annual"`
	Difference *string `json:"difference,omitempty"`
	Valid      bool    `json:"valid"`
}

type CalculationTrace struct {
	Component    string            `json:"component"`
	Rule         string            `json:"rule"`
	Expression   string            `json:"expression"`
	Dependencies []string          `json:"dependencies"`
	Values       map[string]string `json:"values,omitempty"`
}

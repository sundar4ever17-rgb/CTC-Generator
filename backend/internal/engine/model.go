package engine

const Version = "1.0.0"

type Configuration struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Version       int        `json:"version"`
	Source        string     `json:"source"`
	Status        string     `json:"status"`
	Data          ConfigData `json:"data"`
	EffectiveFrom string     `json:"effectiveFrom"`
	EffectiveTo   string     `json:"effectiveTo"`
	CreatedAt     string     `json:"createdAt"`
}
type ConfigData struct {
	AnnualSpecialGroup     string        `json:"annualSpecialGroup"`
	Grades                 []Grade       `json:"grades"`
	Locations              []Location    `json:"locations"`
	BasicMethod            string        `json:"basicMethod"`
	BasicMethodConfirmed   bool          `json:"basicMethodConfirmed"`
	PF                     PFParams      `json:"pf"`
	NPS                    NPSParams     `json:"nps"`
	ESI                    ESIParams     `json:"esi"`
	GratuityRate           string        `json:"gratuityRate"`
	Benefits               []BenefitRule `json:"benefits"`
	Rounding               Rounding      `json:"rounding"`
	AnnualizationConfirmed bool          `json:"annualizationConfirmed"`
	HRAApproved            bool          `json:"hraApproved"`
	TemplateApproved       bool          `json:"templateApproved"`
	OfferTemplate          OfferTemplate `json:"offerTemplate"`
}
type Grade struct {
	ID                    string            `json:"id"`
	Name                  string            `json:"name"`
	BasicMin              *string           `json:"basicMin"`
	BasicMax              *string           `json:"basicMax"`
	ConveyanceCap         *string           `json:"conveyanceCap"`
	Conveyance            *string           `json:"conveyance"`
	ProfessionalAllowance map[string]string `json:"professionalAllowance"`
}
type Location struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type PFParams struct {
	BasicThreshold string `json:"basicThreshold"`
	InclusionShare string `json:"inclusionShare"`
	BasisCap       string `json:"basisCap"`
	Rate           string `json:"rate"`
}
type NPSParams struct {
	OldRate string `json:"oldRate"`
	NewRate string `json:"newRate"`
}
type ESIParams struct {
	CeilingInclusive  *bool   `json:"ceilingInclusive"`
	Rate              string  `json:"rate"`
	Ceiling           *string `json:"ceiling"`
	EligibilityBasis  string  `json:"eligibilityBasis"`
	ContributionBasis string  `json:"contributionBasis"`
}
type Rounding struct {
	Scale     int32  `json:"scale"`
	Mode      string `json:"mode"`
	Confirmed bool   `json:"confirmed"`
}
type OfferTemplate struct {
	CompanyName string `json:"companyName"`
	Address     string `json:"address"`
	Signatory   string `json:"signatory"`
	Body        string `json:"body"`
}
type Condition struct {
	Field  string `json:"field"`
	Equals bool   `json:"equals"`
}
type ScheduleItem struct {
	Year   int    `json:"year"`
	Amount string `json:"amount"`
}
type BenefitRule struct {
	Component   string         `json:"component"`
	Kind        string         `json:"kind"`
	Amount      string         `json:"amount,omitempty"`
	Rate        string         `json:"rate,omitempty"`
	BasisPeriod string         `json:"basisPeriod,omitempty"`
	Basis       string         `json:"basis,omitempty"`
	Period      string         `json:"period"`
	GradeID     string         `json:"gradeId,omitempty"`
	LocationID  string         `json:"locationId,omitempty"`
	Conditions  []Condition    `json:"conditions,omitempty"`
	Schedule    []ScheduleItem `json:"schedule,omitempty"`
	Cap         string         `json:"cap,omitempty"`
	Notes       string         `json:"notes,omitempty"`
}
type Input struct {
	CandidateName      string          `json:"candidateName"`
	CandidateReference string          `json:"candidateReference"`
	Designation        string          `json:"designation"`
	Department         string          `json:"department"`
	JoiningDate        string          `json:"joiningDate"`
	LocationID         string          `json:"locationId"`
	GradeID            string          `json:"gradeId"`
	Qualification      string          `json:"qualification"`
	Gross              string          `json:"gross"`
	Basic              string          `json:"basic"`
	TaxRegime          string          `json:"taxRegime"`
	NPSOptIn           bool            `json:"npsOptIn"`
	Job                map[string]bool `json:"job"`
	CTCYear            int             `json:"ctcYear"`
	Notes              string          `json:"notes"`
}
type Component struct {
	Code  string `json:"code"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

var Components = []Component{
	{"BASIC", "Basic Salary", "inclusion"},
	{"PROFESSIONAL", "Professional Allowance", "inclusion"},
	{"SPECIAL", "Special Allowance", "inclusion"},
	{"HRA", "House Rent Allowance", "exclusion"},
	{"CONVEYANCE", "Conveyance", "exclusion"},
	{"PF", "Employer PF", "exclusion"},
	{"NPS", "NPS", "exclusion"},
	{"SUPERANNUATION", "Superannuation", "exclusion"},
	{"ESI", "Employer ESI", "statutory"},
	{"GRATUITY", "Gratuity", "statutory"},
	{"ANNUAL_SPECIAL", "Annual Special Allowance", "statutory"},
	{"SIXTH_DAY", "6th Day Allowance", "policy"},
	{"FIELD_SERVICE", "Field Service Allowance", "policy"},
	{"RELOCATION", "Relocation", "policy"},
	{"DEPUTATION", "Deputation", "policy"},
	{"ADDITIONAL_HRA", "Additional HRA", "policy"},
	{"PERFORMANCE", "Annual Performance Pay", "variable"},
	{"ADDITIONAL_PERFORMANCE", "Additional Annual Performance Pay", "variable"},
	{"LTM1", "Long Term Merit 1", "variable"},
	{"LTM2", "Long Term Merit 2", "variable"},
}

type Issue struct {
	Code     string `json:"code"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}
type Line struct {
	Code    string  `json:"code"`
	Name    string  `json:"name"`
	Group   string  `json:"group"`
	Monthly *string `json:"monthly"`
	Annual  *string `json:"annual"`
	Amount  *string `json:"amount"`
	Period  string  `json:"period"`
	Status  string  `json:"status"`
	Formula string  `json:"formula"`
}
type Totals struct {
	Inclusion        *string `json:"inclusion"`
	Exclusion        *string `json:"exclusion"`
	Gross            string  `json:"gross"`
	Difference       *string `json:"difference"`
	KnownAnnualTotal string  `json:"knownAnnualTotal"`
	AnnualCTC        *string `json:"annualCTC"`
	StatutoryAnnual  *string `json:"statutoryAnnual"`
	PolicyAnnual     *string `json:"policyAnnual"`
	VariableAnnual   *string `json:"variableAnnual"`
}
type Result struct {
	Lines                []Line  `json:"lines"`
	Totals               Totals  `json:"totals"`
	CoreValid            bool    `json:"coreValid"`
	Complete             bool    `json:"complete"`
	CanFinalize          bool    `json:"canFinalize"`
	Issues               []Issue `json:"issues"`
	ConfigurationID      string  `json:"configurationId"`
	ConfigurationVersion int     `json:"configurationVersion"`
	EngineVersion        string  `json:"engineVersion"`
}

func Str(s string) *string { return &s }

var Clarifications = []map[string]string{
	{"id": "GRADE_MATRIX", "title": "Grade and salary annexure", "detail": "Supply Basic min/max, Conveyance amounts/caps and Professional Allowance slabs. The URD illustration is not an approved grade."},
	{"id": "BASIC_METHOD", "title": "Basic selection", "detail": "Confirm manual selection or provide the approved Gross-based default formula. The demo demonstrates the URD's entered Basic example."},
	{"id": "SUPERANNUATION", "title": "Superannuation policy", "detail": "Supply eligibility, rate and upstream calculation basis. The URD worked example explicitly assumes no Superannuation."},
	{"id": "ESI", "title": "ESI ceiling and wage bases", "detail": "Confirm the ceiling, eligibility wage and contribution wage. Gross includes employer benefits in this URD; do not assume payroll wage equivalence."},
	{"id": "ANNUAL_SPECIAL", "title": "Annual Special Allowance", "detail": "Supply the annual slabs and confirm classification. This is separate from monthly Special Allowance."},
	{"id": "POLICY", "title": "Policy-based payments", "detail": "Define eligibility, amounts/bases, periods and caps for all five policy payments."},
	{"id": "VARIABLE", "title": "Variable pay and LTM", "detail": "Supply grade bands, rates, bases, schedules and annual attribution. No percentage of final CTC is assumed."},
	{"id": "ROUNDING", "title": "Rounding and annualization", "detail": "Approve scale, half-up rounding, Gross input increments and full-year versus first-year annualization. The demonstration uses two decimal places and 12 monthly periods only as a labeled presentation convention."},
	{"id": "HRA", "title": "Finance/Tax HRA approval", "detail": "The URD requires uncapped HRA as an Exclusion residual. Finance/Tax must confirm compensation treatment; this app does not compute tax exemptions."},
	{"id": "TEMPLATE", "title": "Offer template and permissions", "detail": "Supply company-approved offer wording and fields; confirm HR input-edit permissions versus C&B master edit rights."},
}

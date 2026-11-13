package engine

func DemoConfiguration() Configuration {
	benefits := []BenefitRule{{Component: "SUPERANNUATION", Kind: "none", Period: "monthly", Notes: "URD section 4.5 explicitly assumes no Superannuation for this illustration only."}}
	for _, code := range []string{"ANNUAL_SPECIAL", "SIXTH_DAY", "FIELD_SERVICE", "RELOCATION", "DEPUTATION", "ADDITIONAL_HRA", "PERFORMANCE", "ADDITIONAL_PERFORMANCE", "LTM1", "LTM2"} {
		period := ""
		benefits = append(benefits, BenefitRule{Component: code, Kind: "pending", Period: period, Notes: "Not supplied in the URD. Period is an editable draft field, not an approved payment rule."})
	}
	return Configuration{Name: "URD worked example - clarification required", Source: "urd-demo", Status: "published", Data: ConfigData{
		Grades:       []Grade{{ID: "urd-example", Name: "URD illustration (grade unconfirmed)", Conveyance: Str("7300"), ConveyanceCap: Str("7300"), ProfessionalAllowance: map[string]string{}}},
		Locations:    []Location{{ID: "unconfirmed", Name: "Location to be confirmed"}},
		BasicMethod:  "manual",
		PF:           PFParams{BasicThreshold: "25000", InclusionShare: "0.5", BasisCap: "25000", Rate: "0.12"},
		NPS:          NPSParams{OldRate: "0.10", NewRate: "0.14"},
		ESI:          ESIParams{Rate: "0.0325"},
		GratuityRate: "0.0481", Benefits: benefits,
		Rounding:      Rounding{Scale: 2, Mode: "half_up"},
		OfferTemplate: OfferTemplate{CompanyName: "Company name pending approval", Address: "", Signatory: "Authorized signatory to be confirmed", Body: "This is a demonstration offer preview. Employment terms, joining details and compensation provisions require the client's approved template. This document is not an employment offer."},
	}}
}

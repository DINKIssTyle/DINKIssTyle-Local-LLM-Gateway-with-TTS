package evenrealities

import _ "embed"

// This app-owned skill is connected explicitly to isolated hint stages. It
// cannot be displaced by ordinary chat's skill budget or user skill selection.
//
//go:embed skills/trusted-information/SKILL.md
var trustedInformationSkill string

//go:embed skills/trusted-information/references/planning.md
var trustedInformationPlanning string

//go:embed skills/trusted-information/references/selection.md
var trustedInformationSelection string

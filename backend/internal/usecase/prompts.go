package usecase

import _ "embed"

// System prompts for the three agent loops, embedded so the binary is
// self-contained. See agent/prompts/*.md (mirrored under prompts/) for content.
var (
	//go:embed prompts/daily.md
	dailyPrompt string
	//go:embed prompts/followup.md
	followupPrompt string
	//go:embed prompts/monthly.md
	monthlyPrompt string
)

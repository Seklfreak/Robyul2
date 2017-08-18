package triggers

import "github.com/Seklfreak/Robyul2/helpers"

type ReZero struct{}

func (r *ReZero) Triggers() []string {
	return []string{
		"rem",
		"re:zero",
		"rezero",
	}
}

func (r *ReZero) Response(trigger string, content string) string {
	return helpers.GetText("triggers.re_zero.link")
}

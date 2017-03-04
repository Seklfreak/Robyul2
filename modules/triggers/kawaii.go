package triggers

import "github.com/Seklfreak/Robyul2/helpers"

// Kawaii holder for the kawaii command
type Kawaii struct{}

// Triggers cmds
func (k Kawaii) Triggers() []string {
    return []string{
        "kawaii",
    }
}

// Response with a kawaii pic link
func (k Kawaii) Response(trigger string, content string) string {
    return helpers.GetText("triggers.kawaii.link")
}

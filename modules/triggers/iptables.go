package triggers

import "github.com/Seklfreak/Robyul2/helpers"

type IPTables struct{}

func (i *IPTables) Triggers() []string {
    return []string{
        "ipt",
        "iptables",
    }
}

func (i *IPTables) Response(trigger string, content string) string {
    return helpers.GetText("triggers.iptables")
}

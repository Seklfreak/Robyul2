package modules

import (
    "git.lukas.moe/sn0w/Karen/modules/plugins"
    "git.lukas.moe/sn0w/Karen/modules/triggers"
)

var (
    pluginCache  map[string]*Plugin
    triggerCache map[string]*TriggerPlugin

    PluginList = []Plugin{
        &plugins.About{},
        &plugins.Avatar{},
        &plugins.WhoIs{},
        &plugins.Calc{},
        &plugins.Changelog{},
        &plugins.Choice{},
        &plugins.FlipCoin{},
        &plugins.Giphy{},
        &plugins.Google{},
        &plugins.Headpat{},
        &plugins.Leet{},
        &plugins.ListenDotMoe{},
        &plugins.Minecraft{},
        &plugins.Music{},
        &plugins.Osu{},
        &plugins.Ping{},
        &plugins.RandomCat{},
        &plugins.Ratelimit{},
        &plugins.Reminders{},
        &plugins.Roll{},
        &plugins.RPS{},
        &plugins.Stats{},
        &plugins.Stone{},
        &plugins.Support{},
        &plugins.Announcement{},
        //&plugins.Translator{},
        &plugins.Uptime{},
        &plugins.UrbanDict{},
        &plugins.Weather{},
        &plugins.XKCD{},
    }

    TriggerPluginList = []TriggerPlugin{
        &triggers.CSS{},
        &triggers.Donate{},
        &triggers.Git{},
        &triggers.EightBall{},
        &triggers.Hi{},
        &triggers.HypeTrain{},
        &triggers.Invite{},
        &triggers.IPTables{},
        &triggers.Lenny{},
        &triggers.Nep{},
        &triggers.Kawaii{},
        &triggers.ReZero{},
        &triggers.Shrug{},
        &triggers.TableFlip{},
        &triggers.Triggered{},
    }
)

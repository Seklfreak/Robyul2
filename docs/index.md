Disclaimer: This bot is still in a early stage.<br>
Please expect (rare) crashes and performance problems until the bot is mature enough.
<hr>

# Karen &nbsp; ![](http://i.imgur.com/vDJVt9g.png)

Karen is a highly efficient, multipurpose [Discord](https://discordapp.com/) bot written in [Go](http://golang.org/).

A design goal was to keep the experience with her non-intrusive.<br>
That means:

- She will not send a message after you invite her.
- She will not do anything you didn't told her to do
- She will not ask you to set a prefix.
- She will never advertise other servers or bots
- Everyone shall be able to execute any plugin anytime. (see next point)
- There are no plans to create plugins that require admin permissions

<hr>

Got any problems or just want to hang out with me and some other devs?<br>
Join the Discord Server! :)<br>
[![](https://discordapp.com/api/guilds/180818466847064065/widget.png)](https://discord.gg/5SjDr3G)

<hr>

### Invite
Invite link is coming soon.

Want beta-access?

Register here: https://goo.gl/forms/9J9GYMg8c9IM6a5Z2

<hr>

### Karen's Friends :tada:
Bots built by friends or awesome strangers

|Avatar|Name|Language|Link|
|:-|:-|:-|:-|
|![](http://i.imgur.com/SrgZI3g.png)|Emily|Java|[MaikWezinkhof/DiscordBot](https://github.com/MaikWezinkhof/DiscordBot)
|![](http://i.imgur.com/Tb0FZoZ.png)|Shinobu-Chan|Python 3|[Der-Eddy/discord_bot](https://github.com/Der-Eddy/discord_bot)
|![](http://i.imgur.com/PNcNRfM.png)|Ako-Chan|C#|[Serraniel/Ako-Discord-Bot-Loader](https://github.com/Serraniel/Ako-Discord-Bot-Loader)
|![](http://i.imgur.com/7KiL7oG.png)|Rem|Java|[Daniele122898/Rem](https://github.com/Daniele122898/Rem)
|![](http://i.imgur.com/vBnv5u2.png)|Winry Rockbell|JavaScript|[Devsome/Winry-Discordbot](https://github.com/Devsome/EliteBot) <br> **Warning:** Author likes and writes messy code!

<br>

# Docs

The docs are still being written.
Expect incompleteness.

### What to do after inviting
Tell the owner of your server to write a message setting the prefix.<br>
Example:
```
@Karen SET PREFIX !
```
This will make Karen listen for any command that begins with `!`.<br>
Be sure to set it to something no other bot uses on your server.

### Commandlist
|Command|Aliases|Usage|Description|
|:-|:-|:-|:-|
|!about|!a|-|Shows some information about Karen.
|!giphy|!gif|`!gif <search term>`|Searches for gifs on giphy.com
|!google|!goog|`!google <search term>`|Generates a link for someone who's too dumb to google (aka "that guy")
|!invite|!inv|-|Get an invite link for Karen
|!ping|-|-|Shows Karen's current ping to discord
|!cat|-|-|Shows a random cat image
|!remind|!rm|`!remind ordering pizza in 25 <seconds/s/minutes/m/hours/h/days/d>`
|!reminders|!rms|-|Shows your pending reminders
|!roll|-|`!roll 25 80`|Rolls a random number in the given range
|!stats|-|-|Shows stats about Karen
|!stone|-|`!stone @some-user`|Stones someone to death
|!help|!h|-|Shows this help

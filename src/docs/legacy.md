# Shiro &nbsp; ![](https://i.imgur.com/CxYRxt0.png)
[![Dependency Status](https://www.versioneye.com/user/projects/57eb7ac4bd6fa600512e569e/badge.svg?style=flat-square)](https://www.versioneye.com/user/projects/57eb7ac4bd6fa600512e569e)
[![](https://images.microbadger.com/badges/version/sn0w/shiro.svg)](http://microbadger.com/images/sn0w/shiro) 
[![](https://images.microbadger.com/badges/image/sn0w/shiro.svg)](https://microbadger.com/images/sn0w/shiro)

Shiro is a kinda efficient, multipurpose [Discord](https://discordapp.com/) bot written in [Groovy](http://groovy-lang.org/).<br>

A design goal was to keep the experience with her non-intrusive.<br>
That means there are currently neither modules for moderation/administration nor plans to make them.<br>
However since Shiro supports modules you can extend her to infinity!

<hr>
After inviting her set your prefix by sending `@Shiro SET PREFIX <your-prefix>`.<br>
Example: `@Shiro SET PREFIX %`

Got any problems or just want to chat with me and other devs?<br>
Join the Discord Server! :)<br>
[![](https://discordapp.com/api/guilds/180818466847064065/widget.png)](https://discord.gg/5SjDr3G)
<hr>
### Shiro's Friends :tada:
Bots built by friends or awesome strangers

|Avatar|Name|Language|Link|
|:-:|:-:|:-:|:-:|
|![](http://i.imgur.com/SrgZI3g.png)|Emily|Java|[MaikWezinkhof/DiscordBot](https://github.com/MaikWezinkhof/DiscordBot)
|![](http://i.imgur.com/PNcNRfM.png)|Ako-Chan|C#|[Serraniel/Ako-Discord-Bot-Loader](https://github.com/Serraniel/Ako-Discord-Bot-Loader)
|![](http://i.imgur.com/Tb0FZoZ.png)|Shinobu-Chan|Python 3|[Der-Eddy/discord_bot](https://github.com/Der-Eddy/discord_bot) <br> **Warning:** Shiro (anime character) hater
|![](http://i.imgur.com/vBnv5u2.png)|Winry Rockbell|JavaScript|[Devsome/Winry-Discordbot](https://github.com/Devsome/EliteBot) <br> **Warning:** Author likes and writes messy code!
|![](https://i.imgur.com/PlRrEFk.png)|Luna|Python3|[Miraai/LunaBot](https://github.com/miraai/LunaBot)

<hr>

### Can I suggest features/commands/...?
Nope.
Only Karen will receive new features.

### Are you kidding? Java is everything but not efficient...
I am not.<br>
Even while Playing music shiro only consumes about 32mb RAM\* and a few percent CPU.<br>
That's less than one open tab in Google Chrome.<br>
I archieved this by dumping runtime audio conversions.<br>
Shiro utilizes FFMPEG/libav and opusenc to process your audio **before** sending it to discord.<br>

<sub>*\*In-Use heap. Results may vary depending on JVM version and active Garbage Collector.*</sub>
### Requirements
- Any OS and CPU that runs Java 8 [or Docker]
- About 32mb of free RAM
- About 20mb of free HDD space [The docker image needs additional 250mb]
- A MySQL server (anywhere. Maybe at bplaced? 😅)
- FFMPEG/libav, youtube-dl and opusenc if you want to use the Music module
- Internet connection, duh

### Docker? Docker!
Just do a 
```
docker run -dv /docker/shiro:/data --link <mysql-container>:mysql sn0w/shiro:<full commit id or branch name>
```
and everything is ready! :)

### Notable Mentions (<3)
Shiro wouldn't exist without these awesome pieces of software!

- [Groovy by CodeHaus/Apache](http://groovy-lang.org)
- [Discord](http://discordapp.com)
- [Discord4J by austinv11](https://github.com/austinv11/Discord4J)
- [Reflections by Ronmamo](https://github.com/ronmamo/reflections)
- [Unirest by Mashape](http://unirest.io)
- [Chatter-Bot-Api by Pierre David Belanger](https://github.com/pierredavidbelanger/chatter-bot-api)
- [Youtube-DL by RG3](https://github.com/rg3/youtube-dl/)
- [FFMPEG](http://ffmpeg.org/)
- [libav](https://libav.org/)
- [OPUS](https://opus-codec.org/)
- [Minimal JSON by RalfSTX](https://github.com/ralfstx/minimal-json)
- [VorbisJava by Gagravarr](https://github.com/Gagravarr/VorbisJava)
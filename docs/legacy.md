# Shiro &nbsp; ![](https://i.imgur.com/CxYRxt0.png)
[![Dependency Status](https://www.versioneye.com/user/projects/57eb7ac4bd6fa600512e569e/badge.svg?style=flat-square)](https://www.versioneye.com/user/projects/57eb7ac4bd6fa600512e569e)
[![](https://images.microbadger.com/badges/version/sn0w/shiro.svg)](http://microbadger.com/images/sn0w/shiro) 
[![](https://images.microbadger.com/badges/image/sn0w/shiro.svg)](https://microbadger.com/images/sn0w/shiro)

Shiro is a kinda efficient, multipurpose [Discord](https://discordapp.com/) bot written in [Groovy](http://groovy-lang.org/).<br>

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

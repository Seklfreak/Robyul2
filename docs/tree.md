# Family Tree

While developing discord bots my ideas went through many stages and were published with many codenames.
This page is a historically correct list of all bots i've *ever* made.

### BMO (v0) ![](http://i.imgur.com/wMUvnmy.png)
Language: [NodeJS](https://nodejs.org)<br>
Framework: [hydrabolt/discord.js](https://github.com/hydrabolt/discord.js)<br>
Influenced by: -<br>
Influenced: NEPTR<br>
<br>
BMO was my first attempt on writing discord bots.

Even though BMO was never released publicly he boosted my motivation for this topic.<br>
Without him no other bots would *ever* exist.

### NEPTR (v1) ![](http://i.imgur.com/cfd5Szt.gif)
Language: [NodeJS](https://nodejs.org)<br>
Framework: [qeled/discordie](https://github.com/qeled/discordie)<br>
Influenced by: BMO<br>
Influenced: Karen, [Devsome/awesom-o](https://github.com/Devsome/discordBot)<br>
<br>
NEPTR made a framework switch.<br>
While playing with BMO I noticed that the API of `discord.js` was kinda unintuitive and didn't care about JS objects at all.
The API changed since then so **no offense** to hydrabolt here. Discordie just offered a way easier to understand API back then.
The library switch changed a huge part of the code so i felt like this was a good moment for a name-change.

NEPTR was my first bot featuring a plugin-loader.
It wasn't very mature back then but seemingly good enough to influence other bots.
[Devsome/awesom-o](https://github.com/Devsome/discordBot) borrowed major parts of the plugin API and also received some patches from NEPTR.

This was the way to define a plugin back then:
```js
var commands = {
    "ping": {
        usage: "",
        description: "Checking the reaction from the bot",
        cooldown: 15,
        process: function(clientBot, msg) {
            ...code...
        }
    },
    "other_command": {
        ...
    }
```

Even though NEPTR was released and also joined a few discord servers, he didn't receive much attention.

### Karen (v2) ![](http://i.imgur.com/gYDZKvW.png)
Language: [NodeJS](https://nodejs.org)<br>
Framework: [hydrabolt/discord.js](https://github.com/qeled/discordie)<br>
Influenced by: NEPTR<br>
Influenced: [Devsome/Winry](https://github.com/Devsome/Winry-Discordbot)<br>
<br>

Karen was born through the common unhappiness with NEPTR's dusted internals.<br>
She was written completely from scratch and added a more mature plugin loader featuring native CommonJS integration and file-based loading.<br>
The loader was mature enough to influence yet another bot.
[Devsome/Winry](https://github.com/Devsome/Winry-Discordbot) ported major parts of it.

This was the way to define a plugin:
```js
let mod = {
  name: "ping",
  enabled: true,
  on: ["ping", "p"],
  usage: "",
  description: "Checking the reaction from the bot",
  cooldown: 15,
  by: "Author",
  process: function(clientBot, msg) {
    ...code...
  }
};

module.exports = mod;
```

### Shiro (v3) ![](https://i.imgur.com/CxYRxt0.png)
Language: [Groovy](http://groovy-lang.org/)<br>
Framework: [austinv11/Discord4J](https://github.com/austinv11/Discord4J)<br>
Influenced by: Karen, [Serraniel/Ako](https://github.com/Serraniel/Ako-Discord-Bot-Loader)<br>
Influenced: [Daniele/Rem](https://github.com/Daniele122898/Rem), [Serraniel/Ako](https://github.com/Serraniel/Ako-Discord-Bot-Loader)<br>
<br>

While Karen grew bigger and bigger I pushed JavaScript to it's limits.
I wanted to go OOP but without prototype-pollution or pseudo-classes.
Inheritance and forcing external plugin developers to implement certain features just wasn't easily possible.

The easy (but as it later turned out fatal) decision was to use Groovy (a Java dialect).
C# *was* an option but there were some psychological issues on my side (like refusing to type `mono bot.exe` on Linux) and also some other problems (like NuGet being unable to properly handle trasitive dependencies).

Like Karen (v2) she was written completely from scratch and introduced many new concepts and ideas:

- Instead of loading plain objects Shiro was able to search the classpath for hot-swapped classes annotated with `@ShiroMeta()` at runtime.<br>
- Per-guild prefixes
- Database instead of JSON-configs
- Musicbot
- Fastest bot so far
- Most efficient bot so far
- First bot shipped in docker containers
- First auto-deployed bot
- First auto-deployed stage

Here is an example plugin:
```java
package moe.lukas.shiro.modules

import groovy.transform.CompileStatic
import moe.lukas.shiro.annotations.ShiroCommand
import moe.lukas.shiro.annotations.ShiroMeta
import moe.lukas.shiro.core.IModule
import sx.blah.discord.handle.impl.events.MessageReceivedEvent
import sx.blah.discord.handle.obj.IMessage

@ShiroMeta(
    enabled = true,
    description = "Test my reflexes c:",
    commands = [@ShiroCommand(command = "ping")]
)
@CompileStatic
class Ping implements IModule {
    void action(MessageReceivedEvent e) {
        long start = System.nanoTime()
        IMessage message = e.message.channel.sendMessage(":ping_pong: Pong! :grin:")
        message.edit(message.content + " (${(System.nanoTime() - start) / 1000000}ms RTT)")
    }
}
```

Shiro was the most popular bot ever.<br>
As of November 2016 she watched over 100 channels and offered access to more than 600 people.

Shiro's internals were shiny and organized like never before and thus able to influence the API's and features of two bots.
[Daniele/Rem](https://github.com/Daniele122898/Rem) and [Serraniel/Ako](https://github.com/Serraniel/Ako-Discord-Bot-Loader) oriented their features/internals/concepts around her structure/plugins.

### Karen (v4)
> a.k.a. Phoenix from the ashes

Language: [Go](http://golang.org/)<br>
Framework: [bwmarrin/discordgo](https://github.com/bwmarrin/discordgo)<br>

Shiro was groing fast.<br>
Too fast.<br>
At some point I decided to build a music plugin and this spawned an unbelievable amount of problems.<br>
3 bots were dead and now one lay dying.<br>
Someone had to end this mess, so Karen was revived.

Karen is the current effort to build a truly scalable, high-performance discord bot.<br>

C0untLizzi#4250 on the "Coding Lounge" discord server had a major impact on the chosen language and also helps me regulary in understanding it.
HUGE shoutout at this point!
# Contributing

Hi fellow developer!<br>
Thanks for your interest in developing Karen.

In order to get you started, I wrote this little guide that explains how Karen is structured and which tools are used to develop her.

Let's go!

### Requirements

This guide assumes that [git](http://git-scm.com) and [go](http://golang.org)  are already installed and ready to use. Check if everything works and especially if your GOPATH is set-up.

- Hardware
    - A 64bit CPU
    - CPU-Support for AVX2, AVX or SSE3+ instructions is a plus but not (yet) mandatory
- OS
    - Any recent Linux Distro, Windows 7+ or macOS 10.9+
- Shell
    - BASH 4.0 or newer for Linux/macOS users
    - Alternatively any BASH 4.0 compatible shell (like ZSH)
    - PowerShell 2.0 or newer for Windows users
- Software
    -  [git](http://git-scm.com) (Version 2.0 or newer)
    -  [Go](http://golang.org) (Version 1.8 or newer)
- Mandatory Tools
    - [go-bindata](https://github.com/jteeuwen/go-bindata)
    - [Glide](https://glide.sh/)

### Getting Started

Start by pressing the `Fork` button on Gitlab. This creates a copy of my repo that's owned by you and grants you all permissions like push access and web-editing. This will be the place where your code lives until it's merged.

After your fork is created make sure that the folder `$GOPATH/src/git.lukas.moe/sn0w` exists. If not create it.

Now `cd` into this directory and then run `git clone git@git.lukas.moe:<your-username>/Karen.git`. Now you have your own copy of Karen without go complaining about namespace problems.

### Installing dependencies

The depdencies are currently managed with [Glide](https://glide.sh/). It's a tool that works just like npm, cargo, composer and all the other fancy dependency managers - but for go. It's actually a wrapper around Go's vendoring feature but adds critical features like version locking. That's also the reason why you can't `go get` Karen and will most likely never be able to (unless `go dep` leaves the alpha stage one day and manages to do what glide currently does.)

After installing it just `cd` into Karen's folder and run `glide install`.

### Preparing Development

This project uses a git-pattern known as the [GitLab-Flow](https://docs.gitlab.com/ee/university/training/gitlab_flow.html). Please make yourself familiar with it. If you don't comply to the rules it defines your Pull-Request might get rejected.

About Branching:<br>
Karen's development is heavily issue oriented. Ususally you'll see (and use) branch names like this:
```
Issue ID: #12
Issue Title: Fix music plugin

Branch name: 12-fix-music-plugin
```

However if you're developing a new feature I ask you to use an expressive name that follows these patterns:

```
feature/my-awesome-feature

or

bugfix/my-awesome-bugfix
```

If your branch is neither a bugfix or feature nor related to an issue please reach out to me (0xFADED#3237) on Discord. We'll figure something out.

### Development

We're an open and flexible team of developers and don't force anyone into a strict workflow. Just make sure that your editor or IDE has a proper syntax-highlighter and linter. Pull-Requests with syntax errors will be rejected. **Also PLEASE check that your editor doesn't `gofmt` the code on save**. We use their coding-style but have 4 spaces instead of tabs.

Recommended IDE's are Jetbrains Gogland, IntelliJ IDEA, VS-Code and Sublime Text. We can't help you with this topic though. Please don't start working on features before you figured out a workflow that works for you.
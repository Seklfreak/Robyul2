extern crate discord;

use discord::Discord;
use discord::model::Event;
use std::env;

mod modules;
use modules::*;

fn main() {
    print_logo();

    let mods: Vec<Box<KModule>> = vec![
        Box::new(modules::About)
    ];

    for m in &mods {
        println!("Loading module {:?}...", m.name());
        m.init();
    }

    println!("Done");

    let bot = Discord::from_bot_token(
        &env::var("DISCORD_TOKEN").expect("Expecting token in $DISCORD_TOKEN")
    ).expect("Login failed. Wrong token?");

    let (mut connection, _) = bot.connect().expect("Error while connecting to discord!");

    println!("Bot ready. Starting event loop!");

    loop {
        match connection.recv_event() {
            Ok(Event::MessageCreate(message)) => {}

            Ok(_) => {}

            Err(discord::Error::Closed(code, body)) => {
                println!("Gateway closed with code {:?} : {:?}", code, body);
                break;
            }

            Err(err) => println!("Received error: {:?}", err),
        }
    }
}

fn print_logo() {
    let v = vec![
        "",
        "Welcome to...",
        " ▄    ▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄▄▄▄  ▄▄        ▄ ",
        "▐░▌  ▐░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░▌      ▐░▌",
        "▐░▌ ▐░▌ ▐░█▀▀▀▀▀▀▀█░▌▐░█▀▀▀▀▀▀▀█░▌▐░█▀▀▀▀▀▀▀▀▀ ▐░▌░▌     ▐░▌",
        "▐░▌▐░▌  ▐░▌       ▐░▌▐░▌       ▐░▌▐░▌          ▐░▌▐░▌    ▐░▌",
        "▐░▌░▌   ▐░█▄▄▄▄▄▄▄█░▌▐░█▄▄▄▄▄▄▄█░▌▐░█▄▄▄▄▄▄▄▄▄ ▐░▌ ▐░▌   ▐░▌",
        "▐░░▌    ▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░░░░░░░░░░░▌▐░▌  ▐░▌  ▐░▌",
        "▐░▌░▌   ▐░█▀▀▀▀▀▀▀█░▌▐░█▀▀▀▀█░█▀▀ ▐░█▀▀▀▀▀▀▀▀▀ ▐░▌   ▐░▌ ▐░▌",
        "▐░▌▐░▌  ▐░▌       ▐░▌▐░▌     ▐░▌  ▐░▌          ▐░▌    ▐░▌▐░▌",
        "▐░▌ ▐░▌ ▐░▌       ▐░▌▐░▌      ▐░▌ ▐░█▄▄▄▄▄▄▄▄▄ ▐░▌     ▐░▐░▌",
        "▐░▌  ▐░▌▐░▌       ▐░▌▐░▌       ▐░▌▐░░░░░░░░░░░▌▐░▌      ▐░░▌",
        " ▀    ▀  ▀         ▀  ▀         ▀  ▀▀▀▀▀▀▀▀▀▀▀  ▀        ▀▀ ",
        "Version 0.1.0!                                        INDEV!",
        ""
    ];

    for line in v {
        println!("{}", line);
    }
}
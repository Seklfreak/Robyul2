#[macro_use]
extern crate slog;
extern crate discord;

mod modules;
mod core;

use std::env;
use discord::{Discord, State};
use discord::model::Event;
use modules::*;
use core::*;

fn main() {
    ascii::print_logo();

    let mods: Vec<Box<KModule>> = vec![
        Box::new(modules::about::About)
    ];

    for m in &mods {
        println!("Loading module {:?}...", m.name());
        m.init();
    }

    println!("Done");

    let bot = Discord::from_bot_token(
        &env::var("DISCORD_TOKEN").expect("Expecting token in $DISCORD_TOKEN")
    ).expect("Login failed. Wrong token?");

    let (mut connection, ready_event) = bot.connect().expect("Error while connecting to discord!");
    Bot::state = State::new(ready_event);

    println!("Bot ready. Starting event loop!");

    loop {
        match connection.recv_event() {
            Ok(Event::MessageCreate(message)) => {
                let prefix = utils::get_prefix_for_server(message);
            }

            Ok(_) => {}

            Err(discord::Error::Closed(code, body)) => {
                println!("Gateway closed with code {:?} : {:?}", code, body);
                break;
            }

            Err(err) => println!("Received error: {:?}", err),
        }
    }
}
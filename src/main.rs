#[macro_use]
extern crate slog;
extern crate slog_term;
extern crate slog_atomic;
extern crate slog_stream;
extern crate discord;

mod modules;
mod core;

use slog::*;
use std::env;
use discord::{Discord, State};
use discord::model::Event;
use modules::*;
use core::*;

fn main() {
    let logger = Logger::root(
        slog_term::streamer().full().build().fuse(),
        o!()
    );

    info!(logger, "Bootstrapping...");
    ascii::print_logo();

    info!(logger, "Loading modules...");
    let mods: Vec<Box<KModule>> = vec![
        Box::new(modules::about::About)
    ];

    for m in &mods {
        info!(logger, "Initializing"; "Module" => m.name());
        m.init();
    }

    info!(logger, "Done!");

    let bot = Discord::from_bot_token(
        &env::var("DISCORD_TOKEN").expect("Expecting token in $DISCORD_TOKEN")
    ).expect("Login failed. Wrong token?");

    let (mut connection, ready_event) = bot.connect().expect("Error while connecting to discord!");
    //Bot::state = State::new(ready_event);

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
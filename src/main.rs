extern crate discord;

mod modules;
mod utils;

use std::env;
use discord::Discord;
use discord::model::Event;
use modules::*;
use utils::*;

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

    let (mut connection, _) = bot.connect().expect("Error while connecting to discord!");

    println!("Bot ready. Starting event loop!");

    loop {
        match connection.recv_event() {
            Ok(Event::MessageCreate(message)) => {

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
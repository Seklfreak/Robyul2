extern crate discord;

use super::Bot;
use discord::model::Message;
use discord::{State, ChannelRef};

pub fn get_prefix_for_server(message: Message) {
    if let Some(ChannelRef::Public(server, _)) = State::new().find_channel(&message.channel_id) {
        let id = server.id;
    }
}
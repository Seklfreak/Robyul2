use super::KModule;

pub struct About;

impl KModule for About {
    fn name(&self) -> &str { 
        return "About"
    }

    fn info(&self) -> &str { 
        return "Displays information about the bot." 
    }

    fn command(&self) -> &str {
        return "about";
    }

    fn aliases(&self) -> Vec<&str> {
        return vec![]
    }

    fn init(&self) {

    }

    fn action(&self) {
        
    }
}
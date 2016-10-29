/**
 * Export modules
 */
pub mod about;

/**
 * Module "Interface"
 */
pub trait KModule {
    fn name(&self) -> &str;
    fn info(&self) -> &str;
    fn command(&self) -> &str;
    fn aliases(&self) -> Vec<&str>;
    fn init(&self);
    fn action(&self);
}
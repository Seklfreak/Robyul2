/**
 * Export modules
 */
mod about;
pub use self::about::About;

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
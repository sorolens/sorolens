//! Macros shared by the per-resource modules.

/// Generates the asynchronous and blocking methods for one API endpoint.
///
/// Each entry expands into a method on [`crate::Client`] (when the `async`
/// feature is enabled) and on [`crate::BlockingClient`] (when the `blocking`
/// feature is enabled), so the two transports cannot drift apart.
///
/// The path on the right-hand side must point at a `pub(crate)` request
/// builder whose signature is `(config, <method arguments>)` and which returns
/// `Result<`[`RequestSpec`](crate::request::RequestSpec)`, `[`Error`](crate::Error)`)>`.
macro_rules! endpoints {
    ($(
        $(#[$meta:meta])*
        fn $name:ident($($arg:ident : $ty:ty),* $(,)?) -> $ret:ty = $request:path;
    )+) => {
        #[cfg(feature = "async")]
        impl crate::Client {
            $(
                $(#[$meta])*
                pub async fn $name(&self $(, $arg: $ty)*) -> Result<$ret, crate::Error> {
                    let spec = $request(self.config() $(, $arg)*)?;
                    self.send(spec).await
                }
            )+
        }

        #[cfg(feature = "blocking")]
        impl crate::BlockingClient {
            $(
                $(#[$meta])*
                pub fn $name(&self $(, $arg: $ty)*) -> Result<$ret, crate::Error> {
                    let spec = $request(self.config() $(, $arg)*)?;
                    self.send(spec)
                }
            )+
        }
    };
}

pub(crate) use endpoints;

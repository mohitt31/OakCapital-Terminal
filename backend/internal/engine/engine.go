package engine

// Go wrapper around the C++ matching engine — holds a mutex for thread safety, routes orders to CGO, and emits trade/book events to the event bus.

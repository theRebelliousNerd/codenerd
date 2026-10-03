# Event-bus contributor guidance

- Operations needing both event-bus locks acquire `bufferMu` before `mu`.
  Stats and buffered delivery must follow the same order as shutdown.
- Close stops admission, flushes accepted events, joins timer workers, and closes
  owned observers. Late subscriptions are closed; Enable cannot resurrect a bus.
- Keep delivery and dropped-event accounting explicit. Subscribers must not
  block the executive; non-channel sinks retain their nonblocking contract.
- Lock-order tests schedule the actual contention and observe bounded native
  child completion. Race tests alone do not prove deadlock freedom or delivery
  conservation.

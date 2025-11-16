# PRP-001c — FSM Wiring & Fake Adapter

Placeholder for the third slice of PRP-001. Focus: introduce a deterministic fake BotPort, refactor FSM/state packages to depend on the port, and add headless tests that exercise flows using the fake adapter. This slice is also responsible for hydrating `RenderContext.LastPrompt` and `AnswerContext.Message` with the `botport.BotMessage` values returned by the adapters so strategies can consume them transparently.

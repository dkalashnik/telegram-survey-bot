# PRP-001b — Telegram Adapter Extraction

Placeholder for the second slice of PRP-001. Focus: wrap the existing Telegram client with a BotPort adapter, add adapter factory wiring, and keep main.go using the adapter without touching FSM/state yet. The adapter must return fully populated `botport.BotMessage` values (chat/message IDs, transport, payload) so slice 1c can start populating the `RenderContext.LastPrompt` and `AnswerContext.Message` fields without extra refactors.

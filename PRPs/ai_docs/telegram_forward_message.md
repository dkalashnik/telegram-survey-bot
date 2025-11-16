# Telegram Bot API — `forwardMessage`/`forwardMessages` quick reference

Source: <https://core.telegram.org/bots/api#forwardmessage>

## forwardMessage
- Purpose: Forward a single message of any kind. Service/protected messages cannot be forwarded. Returns sent `Message` on success.
- Required params:
  - `chat_id` (int|string): target chat or `@channelusername`.
  - `from_chat_id` (int|string): source chat of the original message.
  - `message_id` (int): ID of the message in `from_chat_id`.
- Optional params:
  - `message_thread_id` (int): target topic in forum supergroups.
  - `direct_messages_topic_id` (int): target topic for direct messages chats.
  - `video_start_timestamp` (int): new start time for forwarded video.
  - `disable_notification` (bool): silent send.
  - `protect_content` (bool): prevent further forwarding/saving.
  - `suggested_post_parameters` (SuggestedPostParameters): only for direct messages chats.

## forwardMessages
- Purpose: Forward multiple messages; skips messages that can’t be found/forwarded. Album grouping preserved. Returns array of `MessageId` on success.
- Required params:
  - `chat_id` (int|string)
  - `from_chat_id` (int|string)
  - `message_ids` (array of int): IDs in `from_chat_id`.
- Optional params mirror `forwardMessage` (`message_thread_id`, `direct_messages_topic_id`, `message_id` for grouping? — per API listing) along with:
  - `disable_notification` (bool)
  - `protect_content` (bool)
  - `scheduling_state` (MessageSchedulingState): schedule delivery.
  - `video_start_timestamp` (int)
  - `suggested_post_parameters` (SuggestedPostParameters)

## Notes for this project
- Our feature likely uses `sendMessage`/`copyMessage` semantics instead of raw forward since we render aggregated text. If we do forward existing messages, ensure `chat_id` is `TARGET_USER_ID` and that protected content is not involved.
- Forwarding can fail for protected content, missing permissions, invalid IDs, or rate limits; handle via BotPort error codes and retain answers on failure.

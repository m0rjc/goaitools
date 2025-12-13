# Research Ideas for Stateful Conversations

Could either of these strategies work:

## Use a Tool to talk to the user. Have the LLM always return the stateful summary

The LLM will have to be told to use a tool to send a reply to the user. Ideally it should be used once at the end of the process.

An advantage here is that we could add more than text. With WhatsApp we could provide buttons and list items too, if the LLM
wants to ask the user a question.

What if the LLM doesn't use the tool?

## Use a Tool to save state

Ask that the LLM call a tool to save a state memento for next time. Return message to the user is the same as normal.
This is the opposite way round to the idea above.

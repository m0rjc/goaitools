# Hello With Tools Example

I want a simple example that:

* Reads `OPENAI_API_KEY` from environment or .env (provide .env.example in this directory)
* Has a game.go which is a model of a simple fake game. It doesn't really make sense, but this game has
   - A title (string)
   - A start date (Date)
   - A duration in minutes (integer)
   - Grid dimensions m,n
* A tool to read the game properties
* A tool to write the game properties if the incoming JSON from the LLM specifies a value for them

The test program provides a system prompt for the game administrator agent that can help the user set up the game.
It performs a sequence of hard coded operations which are easy to see from the short main.go code. 

* It asks about the game.
* It changes the game title
* It changes the start date and duration
* It changes the grid dimensions m,n

For each operation print the user request, the LLM response and the tool log actions.

The system then prints out the end state of the game object.

The aim of this example is both to show how the system works, and to test it with a real LLM.
I believe I have a bug in the system with tool calls being double encoded (perhaps in the marshalling changes we made)
so hope to find that bug while writing the example.

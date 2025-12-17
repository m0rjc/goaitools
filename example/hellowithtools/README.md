# Hello With Tools Example

A simple demonstration of the `goaitools` library that shows how to create AI tools and use them in a conversation loop.

## Overview

This example implements a fake game configuration system with two tools:
- `read_game` - Reads current game properties
- `write_game` - Updates game properties based on AI requests

The game has the following properties:
- **Title** (string)
- **Start Date** (date/time)
- **Duration** (integer minutes)
- **Grid Dimensions** (M x N integers)

## Purpose

This example serves two purposes:

1. **Demonstration** - Shows how to use the `goaitools` library to create tools and integrate them with OpenAI's function calling API
2. **Testing** - Provides an end-to-end test of the system that can be run against a real LLM.

## Setup

1. Copy `.env.example` to `.env`:
   ```bash
   cp .env.example .env
   ```

2. Add your OpenAI API key to `.env`:
   ```
   OPENAI_API_KEY=your-api-key-here
   ```

3. Build the example:
   ```bash
   go build
   ```

4. Run the example:
   ```bash
   ./hellowithtools
   ```

## What It Does

The program executes a sequence of four operations:

1. **Ask about the game** - Uses `read_game` to retrieve current settings
2. **Change game title** - Uses `write_game` to update the title
3. **Change start date and duration** - Uses `write_game` to update multiple properties
4. **Change grid dimensions** - Uses `write_game` to update M and N

For each operation, the program prints:
- The user's request
- Tool actions that were logged (changes made)
- The assistant's response

Finally, it prints the complete game state to verify all changes were applied.

## Expected Output

The program should successfully execute all four operations and show the tool actions logged for each write operation. Read operations don't log actions since they don't modify state.

## Known Issues

Currently, the program fails on operation 2 with "exceeded max tool iterations (10)", indicating a bug in the tool-calling loop where arguments may be double-encoded.

## Code Structure

- `game.go` - Game model definition
- `simple_action.go` - Basic ToolAction implementation for logging
- `read_game_tool.go` - Tool implementation for reading game state
- `write_game_tool.go` - Tool implementation for writing game state
- `main.go` - Main program with test sequence

## Key Patterns Demonstrated

### Tool Action Logging

Tools only log actions that modify state:
- Read operations: No logging
- Write operations: Log each property update
- No-op writes: No logging (if no properties changed)

### Error Handling

Tools use `req.NewErrorResult(err)` to return domain errors to the AI, allowing it to recover or provide feedback to the user.

### JSON Schema Definition

Tools use `aitooling.EmptyJsonSchema()` for tools with no parameters, and `aitooling.MustMarshalJSON()` for compile-time schema definitions.

### Tool Result Creation

All tools return `(*aitooling.ToolResult, error)`:
- Success: `req.NewResult(jsonString), nil`
- Domain error: `req.NewErrorResult(err), nil`
- Infrastructure error: `nil, err`

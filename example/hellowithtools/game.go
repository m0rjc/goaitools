// Package main demonstrates the goaitools library with a simple game configuration system.
//
// This example shows how to create AI tools that read and modify application state,
// with proper action logging and error handling.
package main

import (
	"time"
)

// Game represents a simple fake game with various properties.
// This is a minimal model used to demonstrate tool interactions.
type Game struct {
	Title      string    `json:"title"`
	StartDate  time.Time `json:"start_date"`
	DurationMinutes int   `json:"duration_minutes"`
	GridM      int       `json:"grid_m"`
	GridN      int       `json:"grid_n"`
}

// NewGame creates a new Game with default values
func NewGame() *Game {
	return &Game{
		Title:           "Untitled Game",
		StartDate:       time.Now(),
		DurationMinutes: 60,
		GridM:           10,
		GridN:           10,
	}
}

// Clone creates a copy of the game for transactional updates.
// Since Game contains only value types (string, time.Time, int), a simple struct copy works.
func (g *Game) Clone() *Game {
	clone := *g // Simple struct copy
	return &clone
}

// CommitFrom copies all values from the source game to this game.
// This is used to "commit" changes made to a cloned game back to the original.
func (g *Game) CommitFrom(source *Game) {
	g.Title = source.Title
	g.StartDate = source.StartDate
	g.DurationMinutes = source.DurationMinutes
	g.GridM = source.GridM
	g.GridN = source.GridN
}

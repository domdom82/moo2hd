// Package ai defines the interface that AI faction strategies implement.
// Full strategic decision logic is implemented in a later phase.
package ai

import "context"

// Personality identifies an AI faction's strategic style.
type Personality string

const (
	Expansionist  Personality = "expansionist"
	Militarist    Personality = "militarist"
	Researcher    Personality = "researcher"
	Diplomat      Personality = "diplomat"
	Industrialist Personality = "industrialist"
)

// Faction is the interface all AI personalities implement.
// TakeTurn must return within 1000 ms per faction to keep the 8-faction
// simultaneous-turn server responsive.
type Faction interface {
	Personality() Personality
	TakeTurn(ctx context.Context) error
}

package voting

import "errors"

var (
	// ErrAlreadyVotedThisRound is returned when a patron tries to vote twice in the same round.
	ErrAlreadyVotedThisRound = errors.New("already voted this round")
)

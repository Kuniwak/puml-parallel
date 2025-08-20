package nl

import (
	"github.com/Kuniwak/puml-parallel/lts/pure"
)

type Guard string
type PostCond string

type Edge struct {
	EventID  pure.Event
	StateID  pure.State
	Guard    Guard
	PostCond PostCond
}

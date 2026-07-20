package common

import (
	"crypto/rand"
	"fmt"
)

type ID string

func NewID() ID {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return ID(fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]))
}

func (id ID) String() string {
	return string(id)
}

func (id ID) IsZero() bool {
	return id == ""
}

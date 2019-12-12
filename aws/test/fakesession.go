package test

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

func init() {
	infra.Register("fakesession", new(Session))
}

// Session is fake session provider and should be only used in tests.
type Session struct {
	*session.Session
}

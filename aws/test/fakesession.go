package test

import (
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

func init() {
	infra.Register("fakesession", new(Session))
	infra.Register("fakecreds", new(Creds))
}

// Session is fake session provider and should be only used in tests.
type Session struct {
	*session.Session
}

// Creds are nil credentials (used for testing).
type Creds struct {
	*credentials.Credentials
}

// Init implements infra.Provider.
func (*Creds) Init() error {
	return nil
}

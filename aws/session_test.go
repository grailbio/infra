// Copyright 2019 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.
package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

var schema = infra.Schema{
	"session": new(session.Session),
}

func TestSession(t *testing.T) {
	skipIfNoCreds(t)

	config, err := schema.Make(infra.Keys{
		"session": "github.com/grailbio/infra/aws.Session",
	})
	if err != nil {
		t.Fatal(err)
	}
	var sess1 *session.Session
	config.Must(&sess1)
	if aws.StringValue(sess1.Config.Region) == "" {
		t.Error("empty region")
	}

	p, err := config.Marshal(true)
	if err != nil {
		t.Fatal(err)
	}
	config, err = schema.Unmarshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var sess2 *session.Session
	config.Must(&sess2)

	if got, want := aws.StringValue(sess2.Config.Region), aws.StringValue(sess2.Config.Region); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	creds1, err := sess1.Config.Credentials.Get()
	if err != nil {
		t.Fatal(err)
	}
	creds2, err := sess2.Config.Credentials.Get()
	if err != nil {
		t.Fatal(err)
	}
	if creds1 != creds2 {
		t.Error("credential value mismatch")
	}
}

func skipIfNoCreds(t *testing.T) {
	t.Helper()
	provider := &credentials.ChainProvider{
		VerboseErrors: true,
		Providers: []credentials.Provider{
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{},
		},
	}
	_, err := provider.Retrieve()
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok && aerr.Code() == "NoCredentialProviders" {
			t.Skip("no credentials in environment; skipping")
		}
		t.Fatal(err)
	}
}

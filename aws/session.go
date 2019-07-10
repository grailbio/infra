// Copyright 2019 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// Package AWS defines infrastructure providers for
// AWS configurations.
package aws

import (
	"flag"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

func init() {
	infra.Register("awssession", new(Session))
	infra.Register("awstool", new(AWSTool))
	infra.Register("awscreds", new(AWSCreds))
}

type instance struct {
	Region string            `yaml:"region"`
	Creds  credentials.Value `yaml:"credentials"`
}

// Session is an infrastructure provider for AWS SDK sessions. It
// retrieves AWS configuration and credentials from the environment
// as session.NewSession does (Unix environment variables, or else
// the files ~/.aws/config and ~/.aws/credentials).
//
// Session supports instance marshaling, performed by inlining
// credentials and the session's region. Other configuration
// parameters are currently not propagated by this method.
//
// TODO(marius): copy ~/.aws/config outright when we can? This could
// get complicated in the presence of profiles.
type Session struct {
	*session.Session
	instance instance
}

// Init implements infra.Provider.
func (s *Session) Init() error {
	if s.instance != (instance{}) {
		var err error
		s.Session, err = session.NewSession(&aws.Config{
			Credentials: credentials.NewStaticCredentialsFromCreds(s.instance.Creds),
			Region:      aws.String(s.instance.Region),
		})
		return err
	}

	// session.NewSession() uses a chain provider that looks for
	// credentials first in the environment variables, then in shared
	// credential locations (e.g. ~/.aws/config.yaml), then at remote
	// credential providers (EC2 or ECS roles, ...).  We do not want
	// remote credential providers here, as the credentials are then
	// temporary and cannot be passed to reflowlets, see
	// https://github.com/grailbio/reflow/issues/29. That's why we have
	// a custom chain provider here, without the remote credential
	// providers.
	credProvider := &credentials.ChainProvider{
		VerboseErrors: true,
		Providers: []credentials.Provider{
			&credentials.EnvProvider{},
			&credentials.SharedCredentialsProvider{},
		},
	}
	// We do a retrieval here to catch NoCredentialProviders errors
	// early on.
	if _, err := credProvider.Retrieve(); err != nil {
		return fmt.Errorf("aws.Session: failed to retrieve AWS credentials: %v", err)
	}
	var err error
	s.Session, err = session.NewSessionWithOptions(session.Options{
		Config: aws.Config{
			Credentials: credentials.NewCredentials(credProvider),
		},
		// This loads region configuration from ~/.aws/config.yaml.
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		return err
	}
	s.instance.Region = aws.StringValue(s.Config.Region)
	// Note that the underlying provider may not supply permanent
	// credentials in this case, which could be problematic.
	//
	// TODO(marius): can we at least test for this and warn?
	s.instance.Creds, err = s.Config.Credentials.Get()
	return err
}

// InstanceConfig implements infra.Provider.
func (s *Session) InstanceConfig() interface{} {
	return &s.instance
}

// AWSTool is the awstool docker image name provider.
type AWSTool string

// Flags implements infra.Provider.
func (t *AWSTool) Flags(flags *flag.FlagSet) {
	flags.StringVar((*string)(t), "awstool", "", "Aws tool")
}

// AWSCreds is the permanent AWS credentials provider.
type AWSCreds struct {
	*credentials.Credentials
}

// Init implements infra.Provider.
func (a *AWSCreds) Init(sess *session.Session) error {
	a.Credentials = sess.Config.Credentials
	return nil
}

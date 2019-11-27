package ec2metadata

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

func init() { infra.Register("ec2metadata", new(Session)) }

// EC2Metadata is the infra provider for session.Session using AWS EC2 metadata
type Session struct {
	*session.Session
}

// Help implements infra.Provider
func (Session) Help() string {
	return "use EC2/IAM role credentials"
}

// Init implements infra.Provider
func (e *Session) Init() error {
	var err error
	sess, err := session.NewSession()
	if err != nil {
		return err
	}
	metaClient := ec2metadata.New(sess)
	provider := &ec2rolecreds.EC2RoleProvider{Client: metaClient}
	creds := credentials.NewCredentials(provider)
	doc, err := metaClient.GetInstanceIdentityDocument()
	if err != nil {
		return err
	}

	e.Session, err = session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(doc.Region),
	})
	if err != nil {
		return err
	}
	return err
}

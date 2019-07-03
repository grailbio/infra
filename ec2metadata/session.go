package ec2metadata

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/grailbio/infra"
)

func init() { infra.Register(new(Session)) }

type instance struct {
	Region string            `yaml:"region"`
	Creds  credentials.Value `yaml:"credentials"`
}

// EC2Metadata is the infra provider for session.Session using AWS EC2 metadata
type Session struct {
	*session.Session
	instance instance
}

// Init implements infra.Provider
func (e *Session) Init() error {
	if e.instance != (instance{}) {
		var err error
		e.Session, err = session.NewSession(&aws.Config{
			Credentials: credentials.NewStaticCredentialsFromCreds(e.instance.Creds),
			Region:      aws.String(e.instance.Region),
		})
		return err
	}
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
	e.instance.Region = doc.Region
	// Note that the underlying provider may not supply permanent
	// credentials in this case, which could be problematic.
	//
	// TODO(marius): can we at least test for this and warn?
	e.instance.Creds, err = e.Config.Credentials.Get()
	return err
}

// InstanceConfig implements infra.Provider.
func (e *Session) InstanceConfig() interface{} {
	return &e.instance
}

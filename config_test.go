// Copyright 2018 GRAIL, Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package infra_test

import (
	"errors"
	"flag"
	"testing"

	"github.com/grailbio/base/log"
	"github.com/grailbio/infra"
)

func init() {
	log.AddFlags()
}

type testCreds string

func (c *testCreds) User() string { return string(*c) }

func (*testCreds) Init() error {
	return nil
}

func (c *testCreds) Config() interface{} {
	return c
}

func (c *testCreds) Flags(flags *flag.FlagSet) {
	flags.StringVar((*string)(c), "user", "", "the user name")
}

type clusterInstance struct {
	User string `yaml:"instance_user"`
}

type testCluster struct {
	User         string `yaml:"-"`
	InstanceType string `yaml:"instance_type"`
	NumInstances int    `yaml:"num_instances"`
	SetupUser    string `yaml:"setup_user"`

	FromInstance bool `yaml:"-"`

	instance clusterInstance
}

func (c *testCluster) Init(creds *testCreds) error {
	c.FromInstance = c.instance.User != ""
	if c.FromInstance {
		c.User = c.instance.User
	} else {
		c.User = string(*creds)
	}
	c.instance.User = c.User
	return nil
}

func (c *testCluster) Config() interface{} {
	return c
}

func (c *testCluster) InstanceConfig() interface{} {
	return &c.instance
}

func (c *testCluster) Setup(creds *testCreds) error {
	if string(*creds) == "" {
		return errors.New("no user specified")
	}
	c.InstanceType = "xxx"
	c.NumInstances = 123
	c.SetupUser = string(*creds)
	return nil
}

func (c *testCluster) Version() int {
	return 1
}

type testSetup bool

func (s *testSetup) Setup() error {
	*s = true
	return nil
}

func (s *testSetup) Config() interface{} { return s }

func (*testSetup) Version() int { return 1 }

func init() {
	infra.Register(new(testCreds))
	infra.Register(new(testCluster))
	infra.Register(new(testSetup))
}

var schema = infra.Schema{
	"creds":   new(testCreds),
	"cluster": new(testCluster),
	"setup":   new(testSetup),
}

func TestConfig(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "github.com/grailbio/infra_test.testCreds,user=testuser",
		"cluster": "github.com/grailbio/infra_test.testCluster",
	})
	if err != nil {
		t.Fatal(err)
	}
	var cluster *testCluster
	config.Must(&cluster)
	if got, want := cluster.User, "testuser"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConfigUnmarshal(t *testing.T) {
	config, err := schema.Unmarshal([]byte(`creds: github.com/grailbio/infra_test.testCreds,user=unmarshaled
cluster: github.com/grailbio/infra_test.testCluster
github.com/grailbio/infra_test.testCluster:
  instance_type: xyz
  num_instances: 123
`))
	if err != nil {
		t.Fatal(err)
	}
	var cluster *testCluster
	config.Must(&cluster)
	if got, want := *cluster, (testCluster{"unmarshaled", "xyz", 123, "", false, clusterInstance{"unmarshaled"}}); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestConfigInterface(t *testing.T) {
	type credentials interface {
		User() string
	}
	schema := infra.Schema{"creds": new(credentials)}
	config, err := schema.Make(
		infra.Keys{"creds": "github.com/grailbio/infra_test.testCreds,user=interface"},
	)
	if err != nil {
		t.Fatal(err)
	}
	var creds credentials
	config.Must(&creds)
	if got, want := creds.User(), "interface"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSetup(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "github.com/grailbio/infra_test.testCreds",
		"cluster": "github.com/grailbio/infra_test.testCluster",
		// We include this to make sure that "orphan" providers
		// are also accounted for.
		"setup": "github.com/grailbio/infra_test.testSetup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Setup(); err == nil || err.Error() != "setup github.com/grailbio/infra_test.testCluster: no user specified" {
		t.Fatal(err)
	}
	config, err = schema.Make(infra.Keys{
		"creds":   "github.com/grailbio/infra_test.testCreds,user=xyz",
		"cluster": "github.com/grailbio/infra_test.testCluster",
		"setup":   "github.com/grailbio/infra_test.testSetup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Setup(); err != nil {
		t.Fatal(err)
	}
	// Make sure
	p, err := config.Marshal(false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(p), `cluster: github.com/grailbio/infra_test.testCluster
creds: github.com/grailbio/infra_test.testCreds,user=xyz
github.com/grailbio/infra_test.testCluster:
  instance_type: xxx
  num_instances: 123
  setup_user: xyz
github.com/grailbio/infra_test.testCreds: xyz
github.com/grailbio/infra_test.testSetup: true
setup: github.com/grailbio/infra_test.testSetup
versions:
  github.com/grailbio/infra_test.testCluster: 1
  github.com/grailbio/infra_test.testSetup: 1
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestInstanceConfig(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "github.com/grailbio/infra_test.testCreds,user=testuser",
		"cluster": "github.com/grailbio/infra_test.testCluster",
	})
	if err != nil {
		t.Fatal(err)
	}
	// We don't perform any instantiations before calling Marshal
	// to make sure that this is done properly by the config.
	p, err := config.Marshal(true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(p), `cluster: github.com/grailbio/infra_test.testCluster
creds: github.com/grailbio/infra_test.testCreds,user=testuser
github.com/grailbio/infra_test.testCluster:
  instance_type: ""
  num_instances: 0
  setup_user: ""
github.com/grailbio/infra_test.testCreds: testuser
instances:
  github.com/grailbio/infra_test.testCluster:
    instance_user: testuser
versions: {}
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	config, err = schema.Unmarshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var cluster *testCluster
	config.Must(&cluster)
	if got, want := cluster.User, "testuser"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := cluster.FromInstance, true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

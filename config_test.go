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

type User string

type testUserEmbed struct {
	User
}

func (t *testUserEmbed) Init() error {
	t.User = "embedded"
	return nil
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
	infra.Register("testcreds", new(testCreds))
	infra.Register("testuserembed", new(testUserEmbed))
	infra.Register("testcluster", new(testCluster))
	infra.Register("testsetup", new(testSetup))
	infra.Register("testembedstructcluster", new(testEmbedStructCluster))
	infra.Register("testembeddedcluster", new(TestEmbeddedCluster))
}

var schema = infra.Schema{
	"creds":   new(testCreds),
	"cluster": new(testCluster),
	"setup":   new(testSetup),
}

func TestConfig(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "testcreds,user=testuser",
		"cluster": "testcluster",
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
	config, err := schema.Unmarshal([]byte(`creds: testcreds,user=unmarshaled
cluster: testcluster
testcluster:
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
		infra.Keys{"creds": "testcreds,user=interface"},
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

func TestConfigPromote(t *testing.T) {
	type credentials interface {
		User() string
	}
	schema := infra.Schema{"user": User("")}
	config, err := schema.Make(
		infra.Keys{"user": "testuserembed"},
	)
	if err != nil {
		t.Fatal(err)
	}
	var user User
	config.Must(&user)
	if got, want := string(user), "embedded"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSetup(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "testcreds",
		"cluster": "testcluster",
		// We include this to make sure that "orphan" providers
		// are also accounted for.
		"setup": "testsetup",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := config.Setup(); err == nil || err.Error() != "setup testcluster: no user specified" {
		t.Fatal(err)
	}
	config, err = schema.Make(infra.Keys{
		"creds":   "testcreds,user=xyz",
		"cluster": "testcluster",
		"setup":   "testsetup",
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
	if got, want := string(p), `cluster: testcluster
creds: testcreds,user=xyz
setup: testsetup
testcluster:
  instance_type: xxx
  num_instances: 123
  setup_user: xyz
testcreds: xyz
testsetup: true
versions:
  testcluster: 1
  testcreds: 0
  testsetup: 1
`; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestInstanceConfig(t *testing.T) {
	config, err := schema.Make(infra.Keys{
		"creds":   "testcreds,user=testuser",
		"cluster": "testcluster",
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
	if got, want := string(p), `cluster: testcluster
creds: testcreds,user=testuser
instances:
  testcluster:
    instance_user: testuser
testcluster:
  instance_type: ""
  num_instances: 0
  setup_user: ""
testcreds: testuser
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

type Cluster interface {
	Name() string
}

type TestEmbeddedCluster struct {
	Cluster
	name string
}

func (t *TestEmbeddedCluster) Name() string {
	return t.name
}

type testEmbedStructCluster struct {
	*TestEmbeddedCluster
}

func (t *testEmbedStructCluster) Init() error {
	t.TestEmbeddedCluster = &TestEmbeddedCluster{name: "TestEmbeddedCluster"}
	return nil
}

func TestProviderTypeCoercion(t *testing.T) {
	var schema = infra.Schema{
		"cluster": new(Cluster),
	}
	config, err := schema.Make(infra.Keys{
		"cluster": "testembedstructcluster",
	})
	if err != nil {
		t.Fatal(err)
	}
	var cluster Cluster
	config.Must(&cluster)

	var eCluster *TestEmbeddedCluster
	config.Must(&eCluster)

	var sCluster *testEmbedStructCluster
	config.Must(&sCluster)

	if got, want := cluster.Name(), "TestEmbeddedCluster"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := eCluster.Name(), "TestEmbeddedCluster"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := sCluster.Name(), "TestEmbeddedCluster"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

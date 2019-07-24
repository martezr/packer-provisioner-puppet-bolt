package bolt

import (
	"io/ioutil"
	"os"
  "path"
	"testing"

	"github.com/hashicorp/packer/packer"
)

// Be sure to remove the Puppet Bolt stub file in each test with:
//   defer os.Remove(config["command"].(string))
func testConfig(t *testing.T) map[string]interface{} {
	m := make(map[string]interface{})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	bolt_stub := path.Join(wd, "packer-bolt-stub.sh")

	err = ioutil.WriteFile(bolt_stub, []byte("#!/usr/bin/env bash\necho 1.6.0"), 0777)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	m["command"] = bolt_stub

	return m
}

func TestProvisioner_Impl(t *testing.T) {
	var raw interface{}
	raw = &Provisioner{}
	if _, ok := raw.(packer.Provisioner); !ok {
		t.Fatalf("must be a Provisioner")
	}
}

func TestProvisionerPrepare_Defaults(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

  err := p.Prepare(config)
	if err == nil {
		t.Fatalf("should have error")
	}

	hostkey_file, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkey_file.Name())

	publickey_file, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickey_file.Name())

	config["ssh_host_key_file"] = hostkey_file.Name()
	config["ssh_authorized_key_file"] = publickey_file.Name()
  config["bolt_task"] = "facts"
  config["user"] = "root"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	err = os.Unsetenv("USER")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerPrepare_LocalPort(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkey_file, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkey_file.Name())

	publickey_file, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickey_file.Name())

	config["ssh_host_key_file"] = hostkey_file.Name()
	config["ssh_authorized_key_file"] = publickey_file.Name()

	config["local_port"] = 65537
	err = p.Prepare(config)
	if err == nil {
		t.Fatal("should have error")
	}

	config["local_port"] = 22222
  config["bolt_task"] = "facts"
  config["user"] = "root"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

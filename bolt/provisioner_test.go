package bolt

import (
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
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
	boltStub := path.Join(wd, "packer-bolt-stub.sh")

	err = ioutil.WriteFile(boltStub, []byte("#!/usr/bin/env bash\necho 2.22.0"), 0777)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	m["command"] = boltStub

	return m
}

func TestProvisioner_Impl(t *testing.T) {
	var raw interface{} = &Provisioner{}
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

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()
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

func TestProvisionerPrepare_HostKeyFile(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	filename := make([]byte, 10)
	n, err := io.ReadFull(rand.Reader, filename)
	if n != len(filename) || err != nil {
		t.Fatal("could not create random file name")
	}

	config["ssh_host_key_file"] = fmt.Sprintf("%x", filename)
	config["ssh_authorized_key_file"] = publickeyFile.Name()

	err = p.Prepare(config)
	if err == nil {
		t.Fatal("should error if ssh_host_key_file does not exist")
	}

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["local_port"] = 22222
	config["bolt_task"] = "facts"
	config["user"] = "root"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerPrepare_AuthorizedKeyFiles(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	filename := make([]byte, 10)
	n, err := io.ReadFull(rand.Reader, filename)
	if n != len(filename) || err != nil {
		t.Fatal("could not create random file name")
	}

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = fmt.Sprintf("%x", filename)

	err = p.Prepare(config)
	if err == nil {
		t.Errorf("should error if ssh_authorized_key_file does not exist")
	}

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_authorized_key_file"] = publickeyFile.Name()
	config["local_port"] = 22222
	config["bolt_task"] = "facts"
	config["user"] = "root"
	err = p.Prepare(config)
	if err != nil {
		t.Errorf("err: %s", err)
	}
}

func TestProvisionerPrepare_LocalPort(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()

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

func TestProvisionerPrepare_InventoryDirectory(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()
	config["user"] = "root"
	config["bolt_task"] = "facts"

	config["inventory_directory"] = "doesnotexist"
	err = p.Prepare(config)
	if err == nil {
		t.Errorf("should error if inventory_directory does not exist")
	}

	inventoryDirectory, err := ioutil.TempDir("", "some_inventory_dir")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(inventoryDirectory)

	config["inventory_directory"] = inventoryDirectory
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerPrepare_BoltTask(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()

	err = p.Prepare(config)
	if err == nil {
		t.Fatal("should have error")
	}

	config["user"] = "root"
	config["bolt_task"] = "facts"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerPrepare_LogLevel(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()
	config["bolt_task"] = "facts"
	config["log_level"] = "test"

	err = p.Prepare(config)
	if err == nil {
		t.Fatalf("should error if log_level is not a valid option")
	}

	config["user"] = "root"
	config["bolt_task"] = "facts"
	config["log_level"] = "info"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProvisionerPrepare_RunAs(t *testing.T) {
	var p Provisioner
	config := testConfig(t)
	defer os.Remove(config["command"].(string))

	hostkeyFile, err := ioutil.TempFile("", "hostkey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(hostkeyFile.Name())

	publickeyFile, err := ioutil.TempFile("", "publickey")
	if err != nil {
		t.Fatalf("err: %s", err)
	}
	defer os.Remove(publickeyFile.Name())

	config["ssh_host_key_file"] = hostkeyFile.Name()
	config["ssh_authorized_key_file"] = publickeyFile.Name()
	config["bolt_task"] = "facts"
	config["run_as"] = "root"
	config["backend"] = "winrm"

	err = p.Prepare(config)
	if err == nil {
		t.Fatalf("should error if run_as is set and a non-ssh backend is specified")
	}

	config["user"] = "root"
	config["run_as"] = "root"
	config["backend"] = "ssh"
	config["bolt_task"] = "facts"
	config["log_level"] = "info"
	err = p.Prepare(config)
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestBoltGetVersion(t *testing.T) {
	if os.Getenv("PACKER_ACC") == "" {
		t.Skip("This test is only run with PACKER_ACC=1 and it requires InSpec to be installed")
	}

	var p Provisioner
	p.config.Command = "bolt"
	err := p.getVersion()
	if err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestBoltGetVersionError(t *testing.T) {
	var p Provisioner
	p.config.Command = "./test-fixtures/exit1"
	err := p.getVersion()
	if err == nil {
		t.Fatal("Should return error")
	}
	if !strings.Contains(err.Error(), "./test-fixtures/exit1 --version") {
		t.Fatal("Error message should include command name")
	}
}

func TestCreateInventoryFile(t *testing.T) {
	var p Provisioner

	expectedFile := `---
config:
  winrm:
    user:
      _plugin: env_var
      var: BOLT_USER
    password:
      _plugin: env_var
      var: BOLT_PASSWORD
`
	err := p.createInventoryFile()
	if err != nil {
		t.Fatalf("error creating config using localhost and local port proxy")
	}
	if p.config.InventoryFile == "" {
		t.Fatalf("No inventory file was created")
	}
	defer os.Remove(p.config.InventoryFile)
	f, err := ioutil.ReadFile(p.config.InventoryFile)
	if err != nil {
		t.Fatalf("couldn't read created inventoryfile: %s", err)
	}

	if fmt.Sprintf("%s", f) != expectedFile {
		t.Fatalf("File didn't match expected:\n\n expected: \n%s\n; received: \n%s\n", expectedFile, f)
	}
}

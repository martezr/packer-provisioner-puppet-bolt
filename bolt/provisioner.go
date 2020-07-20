//go:generate mapstructure-to-hcl2 -type Config

package bolt

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/common/adapter"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/template/interpolate"
)

// Config data passed from the template JSON
type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	ctx                 interpolate.Context

	// The command to run bolt
	Command string

	// The Backend to run bolt on
	Backend string `mapstructure:"backend"`

	// The host to run bolt on
	Host string `mapstructure:"host"`

	// The Password to run with bolt. Only used for WinRM
	Password string `mapstructure:"password"`

	// Extra options to pass to the bolt command
	ExtraArguments []string `mapstructure:"extra_arguments"`

	// Bolt environment variables
	BoltEnvVars []string `mapstructure:"bolt_env_vars"`

	// Bolt command parameters
	BoltParams map[interface{}]interface{} `mapstructure:"bolt_params"`

	// The bolt task to execute.
	BoltTask string `mapstructure:"bolt_task"`

	// The bolt plan to execute.
	BoltPlan string `mapstructure:"bolt_plan"`

	// The bolt module path
	BoltModulePath string `mapstructure:"bolt_module_path"`

	// The bolt inventory file
	InventoryFile string `mapstructure:"inventory_file"`

	// Connection Timeout value
	ConnectTimeout int `mapstructure:"connect_timeout"`

	// The directory in which to place the
	//  temporary generated Bolt inventory file. By default, this is the
	//  system-specific temporary file location. The fully-qualified name of this
	//  temporary file will be passed to the `-i` argument of the `Bolt` command
	//  when this provisioner runs Bolt. Specify this if you have an existing
	//  inventory directory with `host_vars` `group_vars` that you would like to
	//  use in the playbook that this provisioner will run.
	InventoryDirectory string `mapstructure:"inventory_directory"`

	LocalPort            int    `mapstructure:"local_port"`
	SkipVersionCheck     bool   `mapstructure:"skip_version_check"`
	User                 string `mapstructure:"user"`
	SSHHostKeyFile       string `mapstructure:"ssh_host_key_file"`
	SSHAuthorizedKeyFile string `mapstructure:"ssh_authorized_key_file"`
	NoWinRMSSLVerify     bool   `mapstructure:"no_winrm_ssl_verify"`
	NoWinRMSSL           bool   `mapstructure:"no_winrm_ssl"`
}

// Provisioner data passed to the provision operation
type Provisioner struct {
	config         Config
	adapter        *adapter.Adapter
	done           chan struct{}
	boltVersion    string
	boltMajVersion uint
}

func (p *Provisioner) ConfigSpec() hcldec.ObjectSpec {
	return p.config.FlatMapstructure().HCL2Spec()
}

// Prepare the config data for provisioning
func (p *Provisioner) Prepare(raws ...interface{}) error {
	p.done = make(chan struct{})

	err := config.Decode(&p.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}

	var errs *packer.MultiError

	// Check that the authorized key file exists
	if len(p.config.SSHAuthorizedKeyFile) > 0 {
		err = validateFileConfig(p.config.SSHAuthorizedKeyFile, "ssh_authorized_key_file", true)
		if err != nil {
			log.Println(p.config.SSHAuthorizedKeyFile, "does not exist")
			errs = packer.MultiErrorAppend(errs, err)
		}
	}
	if len(p.config.SSHHostKeyFile) > 0 {
		err = validateFileConfig(p.config.SSHHostKeyFile, "ssh_host_key_file", true)
		if err != nil {
			log.Println(p.config.SSHHostKeyFile, "does not exist")
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	// Defaults
	if p.config.Command == "" {
		p.config.Command = "bolt"
	}

	// Validate that a bolt task or bolt plan is specified
	if p.config.BoltTask == "" && p.config.BoltPlan == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("A bolt task or bolt plan must be specified"))
	}

	// Validate that both a bolt plan and task are not specified at the same time
	if p.config.BoltTask != "" && p.config.BoltPlan != "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("A bolt task and bolt plan cannot be specified at the same time"))
	}

	if len(p.config.BoltModulePath) > 0 {
		err = validateDirectoryConfig(p.config.BoltModulePath)
		if err != nil {
			log.Println(p.config.BoltModulePath, "does not exist")
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	if !p.config.SkipVersionCheck {
		err = p.getVersion()
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	if p.config.Host == "" {
		p.config.Host = "127.0.0.1"
	}

	if p.config.User == "" {
		usr, err := user.Current()
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		} else {
			p.config.User = usr.Username
		}
	}

	if p.config.User == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("user: could not determine current user from environment"))
	}

	if p.config.LocalPort > 65535 {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("local_port: %d must be a valid port", p.config.LocalPort))
	}

	if len(p.config.InventoryDirectory) > 0 {
		err = validateDirectoryConfig(p.config.InventoryDirectory)
		if err != nil {
			log.Println(p.config.InventoryDirectory, "does not exist")
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}
	return nil
}

func (p *Provisioner) getVersion() error {
	out, err := exec.Command(p.config.Command, "--version").Output()
	if err != nil {
		return fmt.Errorf(
			"Error running \"%s --version\": %s", p.config.Command, err.Error())
	}

	versionRe := regexp.MustCompile(`(\d+\.\d+[.\d+]*)`)
	matches := versionRe.FindStringSubmatch(string(out))
	if matches == nil {
		return fmt.Errorf(
			"Could not find %s version in output:\n%s", p.config.Command, string(out))
	}

	version := matches[1]
	log.Printf("%s version: %s", p.config.Command, version)
	p.boltVersion = version

	majVer, err := strconv.ParseUint(strings.Split(version, ".")[0], 10, 0)
	if err != nil {
		return fmt.Errorf("Could not parse major version from \"%s\"", version)
	}
	p.boltMajVersion = uint(majVer)

	return nil
}

// Provision using the Puppet Bolt provisioner
func (p *Provisioner) Provision(ctx context.Context, ui packer.Ui, comm packer.Communicator, generatedData map[string]interface{}) error {
	ui.Say("Provisioning with Puppet Bolt...")
	p.config.ctx.Data = generatedData

	if p.config.Backend == "" {
		p.config.Backend = generatedData["ConnType"].(string)
	}

	userp, err := interpolate.Render(p.config.User, &p.config.ctx)
	if err != nil {
		return fmt.Errorf("Could not interpolate bolt user: %s", err)
	}

	host, err := interpolate.Render(p.config.Host, &p.config.ctx)
	if err != nil {
		return fmt.Errorf("Could not interpolate bolt host: %s", err)
	}

 	if p.config.Backend == "winrm" {
		host = generatedData["Host"].(string)
		userp = generatedData["User"].(string)
		p.config.Password = generatedData["Password"].(string)

		local_port := 5986
		if p.config.NoWinRMSSL {
			local_port = 5985
		}

		p.config.LocalPort = local_port
	}

	p.config.User = userp
	p.config.Host = host

	k, err := newUserKey(p.config.SSHAuthorizedKeyFile)
	if err != nil {
		return err
	}

	hostSigner, err := newSigner(p.config.SSHHostKeyFile)
	if err != nil {
		return fmt.Errorf("error creating host signer: %s", err)
	}

	// Remove the private key file
	if len(k.privKeyFile) > 0 {
		defer os.Remove(k.privKeyFile)
	}

	keyChecker := ssh.CertChecker{
		UserKeyFallback: func(conn ssh.ConnMetadata, pubKey ssh.PublicKey) (*ssh.Permissions, error) {
			if user := conn.User(); user != p.config.User {
				return nil, errors.New(fmt.Sprintf("authentication failed: %s is not a valid user", user))
			}

			if !bytes.Equal(k.Marshal(), pubKey.Marshal()) {
				return nil, errors.New("authentication failed: unauthorized key")
			}

			return nil, nil
		},
    	IsUserAuthority: func(k ssh.PublicKey) bool { return true },
	}

	config := &ssh.ServerConfig{
		AuthLogCallback: func(conn ssh.ConnMetadata, method string, err error) {
			log.Printf("authentication attempt from %s to %s as %s using %s", conn.RemoteAddr(), conn.LocalAddr(), conn.User(), method)
		},
		PublicKeyCallback: keyChecker.Authenticate,
		//NoClientAuth:      true,
	}

	config.AddHostKey(hostSigner)

	localListener, err := func() (net.Listener, error) {

		port := p.config.LocalPort
		tries := 1
		if port != 0 {
			tries = 10
		}
		for i := 0; i < tries; i++ {
			l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
			port++
			if err != nil {
				ui.Say(err.Error())
				continue
			}
			_, portStr, err := net.SplitHostPort(l.Addr().String())
			if err != nil {
				ui.Say(err.Error())
				continue
			}
			p.config.LocalPort, err = strconv.Atoi(portStr)
			if err != nil {
				ui.Say(err.Error())
				continue
			}
			return l, nil
		}
		return nil, errors.New("Error setting up SSH proxy connection")
	}()

	if err != nil {
		return err
	}

	ui = &packer.SafeUi{
		Sem: make(chan int, 1),
		Ui:  ui,
	}
	p.adapter = adapter.NewAdapter(p.done, localListener, config, "", ui, comm)

	defer func() {
		log.Print("shutting down the SSH proxy")
		close(p.done)
		p.adapter.Shutdown()
	}()

	go p.adapter.Serve()

	if err := p.executeBolt(ui, comm, k.privKeyFile); err != nil {
		return fmt.Errorf("Error executing Bolt: %s", err)
	}

	return nil

}

func (p *Provisioner) executeBolt(ui packer.Ui, comm packer.Communicator, privKeyFile string) error {
	bolttask := p.config.BoltTask
	boltplan := p.config.BoltPlan
	boltmodulepath := p.config.BoltModulePath
	boltparams := p.config.BoltParams

	var envvars []string
	target := fmt.Sprintf("%s://%s:%s", p.config.Backend, p.config.Host, strconv.Itoa(p.config.LocalPort))
	var boltcommand string
	if p.config.BoltTask != "" {
		boltcommand = "task"
	} else {
		boltcommand = "plan"
	}
	args := []string{boltcommand, "run"}
	if p.config.BoltTask != "" {
		args = append(args, bolttask)
	} else {
		args = append(args, boltplan)
	}

	paramData := convertParams(boltparams)
	paramJSON, err := json.Marshal(paramData)
	if err != nil {
		ui.Say(fmt.Sprintf(err.Error()))
	}
	jsonStr := string(paramJSON)

	args = append(args, "--params", jsonStr)

	if p.config.BoltModulePath != "" {
		args = append(args, "--modulepath", boltmodulepath)
	}

	args = append(args, "--targets", target)
	args = append(args, "--user", p.config.User)

	if p.config.Backend == "ssh" {
		args = append(args, "--no-host-key-check")
		args = append(args, "--private-key", privKeyFile)
	} else if p.config.Backend == "winrm" {
		if p.config.InventoryFile == "" {
			err := p.createInventoryFile()
		    if err != nil {
		      return err
		    }

			defer os.Remove(p.config.InventoryFile)
		}

		args = append(args, "--inventoryfile", p.config.InventoryFile)

		envvars = append(envvars, fmt.Sprintf("BOLT_PASSWORD=%s", p.config.Password))
		envvars = append(envvars, fmt.Sprintf("BOLT_USER=%s", p.config.User))

		if p.config.NoWinRMSSL {
			args = append(args, "--no-ssl")
		}

		if p.config.NoWinRMSSLVerify {
			args = append(args, "--no-ssl-verify")
		}
	} else {
		return fmt.Errorf("Backend must be either SSH or Winrm. Given: %s", p.config.Backend)
	}

	if p.config.ConnectTimeout > 0 {
		args = append(args, "--connect-timeout", strconv.Itoa(p.config.ConnectTimeout))
	}

	// expose packer_http_addr extra variable
	httpAddr := common.GetHTTPAddr()
	if httpAddr != "" {
		args = append(args, "--extra-vars", fmt.Sprintf("packer_http_addr=%s", httpAddr))
	}

	args = append(args, p.config.ExtraArguments...)
	if len(p.config.BoltEnvVars) > 0 {
		envvars = append(envvars, p.config.BoltEnvVars...)
	}

	cmd := exec.Command(p.config.Command, args...)

	cmd.Env = os.Environ()
	if len(envvars) > 0 {
		cmd.Env = append(cmd.Env, envvars...)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	repeat := func(r io.ReadCloser) {
		reader := bufio.NewReader(r)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				line = strings.TrimRightFunc(line, unicode.IsSpace)
				ui.Message(line)
			}
			if err != nil {
				if err == io.EOF {
					break
				} else {
					ui.Error(err.Error())
					break
				}
			}
		}
		wg.Done()
	}
	wg.Add(2)
	go repeat(stdout)
	go repeat(stderr)

	ui.Say(fmt.Sprintf("Executing Bolt: %s", strings.Join(cmd.Args, " ")))
	if err := cmd.Start(); err != nil {
		return err
	}
	wg.Wait()
	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("Non-zero exit status: %s", err)
	}

	return nil
}

const WinRMInventory = `---
config:
  winrm:
    user:
      _plugin: env_var
      var: BOLT_USER
    password:
      _plugin: env_var
      var: BOLT_PASSWORD
`

func (p *Provisioner) createInventoryFile() error {
	log.Printf("Creating inventory file for Bolt WinRM run...")
	tf, err := ioutil.TempFile(p.config.InventoryDirectory, "packer-provisioner-bolt.*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file for generated key")
	}

	w := bufio.NewWriter(tf)
	w.WriteString(WinRMInventory)

	if err := w.Flush(); err != nil {
	tf.Close()
		os.Remove(tf.Name())
		return fmt.Errorf("Error preparing packer attributes file: %s", err)
	}

	err = tf.Close()
	if err != nil {
		return fmt.Errorf("failed to write private key to temp file")
	}

	p.config.InventoryFile = tf.Name()

	return nil
}

func convertParams(m map[interface{}]interface{}) map[string]interface{} {
	res := map[string]interface{}{}
	for k, v := range m {
		switch v2 := v.(type) {
		case map[interface{}]interface{}:
			res[fmt.Sprint(k)] = convertParams(v2)
		default:
			res[fmt.Sprint(k)] = v
		}
	}
	return res
}

func validateFileConfig(name string, config string, req bool) error {
	if req {
		if name == "" {
			return fmt.Errorf("%s must be specified.", config)
		}
	}
	info, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("%s: %s is invalid: %s", config, name, err)
	} else if info.IsDir() {
		return fmt.Errorf("%s: %s must point to a file", config, name)
	}
	return nil
}


func validateDirectoryConfig(name string) error {
	info, err := os.Stat(name)
	if err != nil {
		return fmt.Errorf("Directory: %s is invalid: %s", name, err)
	} else if !info.IsDir() {
		return fmt.Errorf("Directory: %s must point to a directory", name)
	}
	return nil
}

type userKey struct {
	ssh.PublicKey
	privKeyFile string
}

func newUserKey(pubKeyFile string) (*userKey, error) {
	userKey := new(userKey)
	if len(pubKeyFile) > 0 {
		pubKeyBytes, err := ioutil.ReadFile(pubKeyFile)
		if err != nil {
			return nil, errors.New("Failed to read public key")
		}
		userKey.PublicKey, _, _, _, err = ssh.ParseAuthorizedKey(pubKeyBytes)
		if err != nil {
			return nil, errors.New("Failed to parse authorized key")
		}

		return userKey, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("Failed to generate key pair")
	}
	userKey.PublicKey, err = ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, errors.New("Failed to extract public key from generated key pair")
	}

	// To support Bolt calling back to us we need to write
	// this file down
	privateKeyDer := x509.MarshalPKCS1PrivateKey(key)
	privateKeyBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateKeyDer,
	}
	tf, err := ioutil.TempFile("", "packer-provisioner-bolt.*.key")
	if err != nil {
		return nil, errors.New("failed to create temp file for generated key")
	}
	_, err = tf.Write(pem.EncodeToMemory(&privateKeyBlock))
	if err != nil {
		return nil, errors.New("failed to write private key to temp file")
	}

	err = tf.Close()
	if err != nil {
		return nil, errors.New("failed to close private key temp file")
	}
	userKey.privKeyFile = tf.Name()

	return userKey, nil
}

type signer struct {
	ssh.Signer
}

func newSigner(privKeyFile string) (*signer, error) {
	signer := new(signer)

	if len(privKeyFile) > 0 {
		privateBytes, err := ioutil.ReadFile(privKeyFile)
		if err != nil {
			return nil, errors.New("Failed to load private host key")
		}

		signer.Signer, err = ssh.ParsePrivateKey(privateBytes)
		if err != nil {
			return nil, errors.New("Failed to parse private host key")
		}

		return signer, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, errors.New("Failed to generate server key pair")
	}

	signer.Signer, err = ssh.NewSignerFromKey(key)
	if err != nil {
		return nil, errors.New("Failed to extract private key from generated key pair")
	}

	return signer, nil
}
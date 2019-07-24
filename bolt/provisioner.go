package bolt

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/crypto/ssh"

	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/common/adapter"
	"github.com/hashicorp/packer/helper/config"
	"github.com/hashicorp/packer/packer"
	"github.com/hashicorp/packer/packer/tmp"
	"github.com/hashicorp/packer/template/interpolate"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	ctx                 interpolate.Context

	// The command to run bolt
	Command string

	// Extra options to pass to the bolt command
	ExtraArguments []string `mapstructure:"extra_arguments"`

	// The bolt task to execute.
	BoltTask string `mapstructure:"bolt_task"`

	// The bolt plan to execute.
	BoltPlan string `mapstructure:"bolt_plan"`

	// The bolt module path
	BoltModulePath string `mapstructure:"bolt_module_path"`

	// The optional inventory file
	InventoryFile        string `mapstructure:"inventory_file"`
	LocalPort            int    `mapstructure:"local_port"`
	SkipVersionCheck     bool   `mapstructure:"skip_version_check"`
	User                 string `mapstructure:"user"`
	SSHHostKeyFile       string `mapstructure:"ssh_host_key_file"`
	SSHAuthorizedKeyFile string `mapstructure:"ssh_authorized_key_file"`
}

type Provisioner struct {
	config         Config
	adapter        *adapter.Adapter
	done           chan struct{}
	boltVersion    string
	boltMajVersion uint
}

type PassthroughTemplate struct {
	WinRMPassword string
}

func (p *Provisioner) Prepare(raws ...interface{}) error {
	p.done = make(chan struct{})

	// Create passthrough for winrm password so we can fill it in once we know
	// it
	p.config.ctx.Data = &PassthroughTemplate{
		WinRMPassword: `{{.WinRMPassword}}`,
	}

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

	// Defaults
	if p.config.Command == "" {
		p.config.Command = "bolt"
	}

	if !p.config.SkipVersionCheck {
		err = p.getVersion()
		if err != nil {
			errs = packer.MultiErrorAppend(errs, err)
		}
	}

	if p.config.User == "" {
		errs = packer.MultiErrorAppend(errs, fmt.Errorf("user: could not determine current user from environment."))
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

	versionRe := regexp.MustCompile(`\w (\d+\.\d+[.\d+]*)`)
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
		return fmt.Errorf("Could not parse major version from \"%s\".", version)
	}
	p.boltMajVersion = uint(majVer)

	return nil
}

// Provision using the Puppet Bolt provisioner
func (p *Provisioner) Provision(ctx context.Context, ui packer.Ui, comm packer.Communicator) error {
	ui.Say("Provisioning with Puppet Bolt...")

	k, err := newUserKey(p.config.SSHAuthorizedKeyFile)
	if err != nil {
		return err
	}

	hostSigner, err := newSigner(p.config.SSHHostKeyFile)
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

// Cancel the provision operation
func (p *Provisioner) Cancel() {
	if p.done != nil {
		close(p.done)
	}
	if p.adapter != nil {
		p.adapter.Shutdown()
	}
	os.Exit(0)
}

func (p *Provisioner) executeBolt(ui packer.Ui, comm packer.Communicator, privKeyFile string) error {
	//	inventory := p.config.InventoryFile
	bolttask := p.config.BoltTask
	boltplan := p.config.BoltPlan
	boltmodulepath := p.config.BoltModulePath

	//	var envvars []string
	target := "ssh://127.0.0.1:" + strconv.Itoa(p.config.LocalPort)
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

	if p.config.BoltModulePath != "" {
		args = append(args, "--modulepath", boltmodulepath)
	}

	args = append(args, "--nodes", target)
	args = append(args, "--no-host-key-check")
	args = append(args, "--user", p.config.User)
	args = append(args, "--private-key", privKeyFile)
	//	if len(privKeyFile) > 0 {
	// Changed this from using --private-key to supplying -e bolt_ssh_private_key_file as the latter
	// is treated as a highest priority variable, and thus prevents overriding by dynamic variables
	// as seen in #5852
	// args = append(args, "--private-key", privKeyFile)
	//		args = append(args, fmt.Sprintf("--private-key %s", privKeyFile))
	//	}

	// expose packer_http_addr extra variable
	httpAddr := common.GetHTTPAddr()
	if httpAddr != "" {
		args = append(args, "--extra-vars", fmt.Sprintf("packer_http_addr=%s", httpAddr))
	}

	args = append(args, p.config.ExtraArguments...)
	//	if len(p.config.BoltEnvVars) > 0 {
	//		envvars = append(envvars, p.config.BoltEnvVars...)
	//	}

	cmd := exec.Command(p.config.Command, args...)

	cmd.Env = os.Environ()
	//	if len(envvars) > 0 {
	//		cmd.Env = append(cmd.Env, envvars...)
	//	}

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

	// remove winrm password from command, if it's been added
	flattenedCmd := strings.Join(cmd.Args, " ")
	sanitized := flattenedCmd
	//	if len(getWinRMPassword(p.config.PackerBuildName)) > 0 {
	//		sanitized = strings.Replace(sanitized,
	//			getWinRMPassword(p.config.PackerBuildName), "*****", -1)
	//	}
	ui.Say(fmt.Sprintf("Executing Bolt: %s", sanitized))

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
	tf, err := tmp.File("bolt-key")
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

func getWinRMPassword(buildName string) string {
	winRMPass, _ := commonhelper.RetrieveSharedState("winrm_password", buildName)
	packer.LogSecretFilter.Set(winRMPass)
	return winRMPass
}

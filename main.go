package main

import (
	"github.com/hashicorp/packer/packer/plugin"
	"github.com/martezr/packer-provisioner-puppet-bolt/bolt"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterProvisioner(new(bolt.Provisioner))
	server.Serve()
}

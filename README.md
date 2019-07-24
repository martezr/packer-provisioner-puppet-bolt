# packer-provisioner-puppet-bolt

[![Build Status](https://img.shields.io/travis/martezr/packer-provisioner-puppet-bolt/master.svg)][travis]
[![GoReportCard][report-badge]][report]


[travis]: https://travis-ci.org/martezr/packer-provisioner-puppet-bolt

[report-badge]: https://goreportcard.com/badge/github.com/martezr/packer-provisioner-puppet-bolt
[report]: https://goreportcard.com/report/github.com/martezr/packer-provisioner-puppet-bolt

Packer Puppet Bolt Provisioner

```
docker run --rm -ti -v $(pwd):/go/src/ golang /bin/bash
```

The bolt Packer provisioner runs Puppet Bolt tasks. It runs an SSH server, executes bolt task run, and marshals Bolt tasks through the SSH server to the machine being provisioned by Packer.

Note:: Any remote_user defined in tasks will be ignored. Packer will always connect with the user given in the json config for this provisioner.

Usage
======

» Basic Example
This is a fully functional template that will provision an image on DigitalOcean. Replace the mock api_token value with your own.

```json
{
  "builders": [
    {
      "type": "digitalocean",
      "api_token": "6a561151587389c7cf8faa2d83e94150a4202da0e2bad34dd2bf236018ffaeeb",
      "image": "ubuntu-14-04-x64",
      "region": "sfo1"
    }
  ],
  "provisioners": [
    {
      "type": "puppet-bolt",
      "user": "root",
      "bolt_plan":"boltdemo::consul_server",
      "bolt_module_path": "modules/"
    }
  ]
}
```

Configuration Reference
======

required parameters
------

- bolt_task (string) - The bolt task to be run.

Optional Parameters:
------

- inventory_file (string) - The inventory file to use during provisioning. When unspecified, Packer will create a temporary inventory file and will use the host_alias.

- local_port (uint) - The port on which to attempt to listen for SSH connections. This value is a starting point. The provisioner will attempt listen for SSH connections on the first available of ten ports, starting at local_port. A system-chosen port is used when local_port is missing or empty.

- ssh_host_key_file (string) - The SSH key that will be used to run the SSH server on the host machine to forward commands to the target machine. Bolt connects to this server and will validate the identity of the server using the system known_hosts. The default behavior is to generate and use a onetime key.

- ssh_authorized_key_file (string) - The SSH public key of the Bolt ssh_user. The default behavior is to generate and use a onetime key. If this key is generated, the corresponding private key is passed to bolt with the --private-key-file option.

- user (string) - The bolt_user to use. Defaults to the user running packer.

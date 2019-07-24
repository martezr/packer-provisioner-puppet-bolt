# packer-provisioner-puppet-bolt

[![Build Status](https://img.shields.io/travis/martezr/packer-provisioner-puppet-bolt/master.svg)][travis]
[![GoReportCard][report-badge]][report]
[![GitHub release](https://img.shields.io/github/release/martezr/packer-provisioner-puppet-bolt.svg)](https://github.com/martezr/packer-provisioner-puppet-bolt/releases/)
[![license](https://img.shields.io/github/license/martezr/packer-provisioner-puppet-bolt.svg)](https://github.com/martezr/packer-provisioner-puppet-bolt/blob/master/LICENSE)


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

» Linux vSphere Example
This is a complete Linux reference template for VMware vSphere.

```json
{
  "variables": {
    "vsphere_password": "",
    "ssh_password": ""
  },
  "builders": [
    {
      "type": "vsphere-clone",

      "vcenter_server":      "10.0.0.205",
      "username":            "administrator@vsphere.local",
      "password":            "{{user `vsphere_password`}}",
      "insecure_connection": "true",

      "template": "centos7base",
      "vm_name":  "alpine-clone",
      "cluster": "GRT-Cluster",
      "host": "10.0.0.246",

      "communicator": "ssh",
      "ssh_username": "root",
      "ssh_password": "{{user `ssh_password`}}"
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

» Windows vSphere Example
This is a complete Windows reference template for VMware vSphere.

```json
{
  "variables": {
    "vsphere_password": "",
    "winrm_password": ""
  },
  "builders": [
    {
      "type": "vsphere-clone",

      "vcenter_server":      "10.0.0.205",
      "username":            "administrator@vsphere.local",
      "password":            "{{user `vsphere_password`}}",
      "insecure_connection": "true",

      "template": "grt2k16temp",
      "vm_name":  "winboltpacker",
      "cluster": "GRT-Cluster",
      "host": "10.0.0.246",

      "communicator": "winrm",
      "winrm_port": "5985",
      "winrm_insecure": true,
      "winrm_username": "administrator",
      "winrm_password": "{{user `winrm_password`}}"
    }
  ],
  "provisioners": [
    {
      "type": "puppet-bolt",
      "user": "administrator",
      "bolt_task": "facts"
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

## License

The provisioner is available as open source under the terms of the [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0).

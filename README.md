Packer Puppet Bolt Provisioner
=======

[![Build Status](https://img.shields.io/travis/martezr/packer-provisioner-puppet-bolt/master.svg)][travis]
[![GoReportCard][report-badge]][report]
[![GitHub release](https://img.shields.io/github/release/martezr/packer-provisioner-puppet-bolt.svg)](https://github.com/martezr/packer-provisioner-puppet-bolt/releases/)
[![license](https://img.shields.io/github/license/martezr/packer-provisioner-puppet-bolt.svg)](https://github.com/martezr/packer-provisioner-puppet-bolt/blob/master/LICENSE)

[travis]: https://travis-ci.org/martezr/packer-provisioner-puppet-bolt

[report-badge]: https://goreportcard.com/badge/github.com/martezr/packer-provisioner-puppet-bolt
[report]: https://goreportcard.com/report/github.com/martezr/packer-provisioner-puppet-bolt

HashiCorp Packer plugin that provisions machines using [Puppet Bolt](https://puppet.com/products/bolt)

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

or

- bolt_plan (string) - The bolt plan to be run.

Optional Parameters:
------

- bolt_module_path (string) - The path that Bolt should look for modules.

- inventory_file (string) - The inventory file to use during provisioning. When unspecified, Packer will create a temporary inventory file and will use the host_alias.

- local_port (uint) - The port on which to attempt to listen for SSH connections. This value is a starting point. The provisioner will attempt listen for SSH connections on the first available of ten ports, starting at local_port. A system-chosen port is used when local_port is missing or empty.

- ssh_host_key_file (string) - The SSH key that will be used to run the SSH server on the host machine to forward commands to the target machine. Bolt connects to this server and will validate the identity of the server using the system known_hosts. The default behavior is to generate and use a onetime key.

- ssh_authorized_key_file (string) - The SSH public key of the Bolt ssh_user. The default behavior is to generate and use a onetime key. If this key is generated, the corresponding private key is passed to bolt with the --private-key-file option.

- user (string) - The bolt_user to use. Defaults to the user running packer.

## License

|                |                                                  |
| -------------- | ------------------------------------------------ |
| **Author:**    | Martez Reed (<martez.reed@greenreedtech.com>)    |
| **Copyright:** | Copyright (c) 2018-2019 Green Reed Technology    |
| **License:**   | Apache License, Version 2.0                      |

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
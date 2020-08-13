## 0.3.0
### August 13th, 2020

IMPROVEMENTS:

* The documentation has been improved to include more of the available parameters and group them by backend type. [[GH-6](https://github.com/martezr/packer-provisioner-puppet-bolt/issues/6)]
* Add support for `run_as` for SSH connections to enable privilege escalation. [[GH-12](https://github.com/martezr/packer-provisioner-puppet-bolt/issues/12)]
* Add support for `log_level` to be able to specify different levels of logging. This is especially useful for troubleshooting build failures during Bolt execution. [[GH-13](https://github.com/martezr/packer-provisioner-puppet-bolt/issues/13)]
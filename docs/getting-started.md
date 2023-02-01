# Installation

The easiest way is to run `pasta` as a container service or get the pre-build binaries from the releases within this repository.

If you prefer the native applications, checkout the sections below.

## Install on openSUSE

openSUSE packages are provided at [build.opensuse.org](https://build.opensuse.org/package/show/home%3Aph03nix%3Atools/pasta).
To install follow the instructions from [software.opensuse.org](https://software.opensuse.org/download/package?package=pasta&project=home%3Aph03nix%3Atools) or the following snippet:

	# Tumbleweed
    zypper addrepo zypper addrepo https://download.opensuse.org/repositories/home:ph03nix:tools/openSUSE_Tumbleweed/home:ph03nix:tools.repo
    zypper refresh && zypper install pasta

## RancherOS

Let's assume we have `/dev/sda` for the system and `/dev/sdb` for data.

* Prepare persistent storage for data
* Install the system with given [`cloud-init.yaml`](cloud-init.yaml.example) to system storage
* Configure your proxy and enojoy!

```bash
$ sudo parted /dev/sdb
  # mktable - gpt - mkpart - 1 - 0% - 100%
$ sudo mkfs.ext4 /dev/sdb1
$ sudo ros install -d /dev/sda -c cloud-init.yaml
```

![Build status badge](https://github.com/grisu48/pasta/workflows/pastad/badge.svg)

# pasta

Stupid simple pastebin service written in go.

Fastest way of deploying is via the [`deploy_pasta.sh`](deploy_pasta.sh) script.

## Run via docker

The easiest way of self-hosting a `pasta` server is via the [docker image](https://hub.docker.com/r/grisu48/pasta/). All you need to do is

* Create your `data` directory
* Put the [pastad.toml](pastad.toml.example) file there
* Start the container, mount the `data` directory as `/data`

Assuming your data is in `/srv/pasta/` you can do

    docker container run -d -v /srv/pasta:/data -p 127.0.0.1:8199:8199 grisu48/pasta

Configure your reverse proxy (e.g. `nginx`) then accordingly. I don't recomment to publish `pastad` on port 80 without a reverse proxy.

## Run on RancherOS

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

## Run on openSUSE

We build openSUSE package at [build.opensuse.org](https://build.opensuse.org/package/show/home:pdostal/pasta). You can install it like this:

    zypper addrepo https://download.opensuse.org/repositories/home:/pdostal/openSUSE_Tumbleweed/home:pdostal.repo
    zypper refresh; zypper install pasta

## Build and run from source

    make pastad                                    # Server
    make pasta                                     # Client
    make                                           # all

Then create a `pastad.toml` file using the provided example (`pastad.toml.example`) and run the server with

    ./pastad

### Build docker image

    make docker

Or manually:

    docker build . -t feldspaten.org/pasta         # Build docker container

Create or run the container with

    docker container create --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta
    docker container run --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta

The container needs a `data` directory with a valid `pastad.toml` (See the [example file](pastad.toml.example), otherwise default values will be used).

# Usage

Assuing the server runs on http://localhost:8199, you can use the `pasta` tool or simply `curl` :

    curl -X POST 'http://localhost:8199' --data-binary @README.md

## pasta CLI

`pasta` is the CLI utility for easy handling. For instance, if you want to push the `README.md` file and create a pasta out of it:

    pasta README.md
    pasta -r http://localhost:8199 REAME.md

`pasta` reads the `~/.pasta.toml` file (see the [example file](pasta.toml.example))

![Build status badge](https://github.com/grisu48/pasta/workflows/pastad/badge.svg)

# pasta

Stupid simple pastebin service written in go.

## Run via podman/docker

The easiest way of self-hosting a `pasta` server is via the provided container from `ghcr.io/grisu48/pasta:latest`. The container runs fine as rootless container. Setup your own `pasta` instance is as easy as:

* Create your `data` directory (holds config + data)
* Create a [pastad.toml](pastad.toml.example) file therein
* Start the container, mount the `data` directory as `/data` and publish port `8199`
* Configure your reverse proxy (e.g. `nginx`) to forward requests to the `pasta` container

Assuming you want your data directory be e.g. `/srv/pasta`, prepare your server:

    mkdir /srv/pasta
    cp pastad.toml.example /srv/pastsa/pastad.toml
    $EDITOR /srv/pastsa/pastad.toml                     # Modify the configuration to your needs

And then create and run your container with your preferred container engine:

    docker container run -d --name pasta -v /srv/pasta:/data -p 127.0.0.1:8199:8199 ghcr.io/grisu48/pasta
    podman container run -d --name pasta -v /srv/pasta:/data -p 127.0.0.1:8199:8199 ghcr.io/grisu48/pasta

`pasta` listens here on port 8199 and all you need to do is to configure your reverse proxy (e.g. `nginx`) accordingly:

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name my-awesome-pasta.server;

    client_max_body_size 32M;
    location / {
        proxy_pass http://127.0.0.1:8199/;
    }
}
```
 
 Note that the good old [dockerhub image](https://hub.docker.com/r/grisu48/pasta/) is deprecated. It still gets updates but will be removed one fine day.
## Run on openSUSE

We build openSUSE package at [build.opensuse.org](https://build.opensuse.org/package/show/home%3Aph03nix%3Atools/pasta). To install follow the instructions from [software.opensuse.org](https://software.opensuse.org/download/package?package=pasta&project=home%3Aph03nix%3Atools) or the following snippet:

	# Tumbleweed
    zypper addrepo zypper addrepo https://download.opensuse.org/repositories/home:ph03nix:tools/openSUSE_Tumbleweed/home:ph03nix:tools.repo
    zypper refresh && zypper install pasta

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

## Build and run from source

    make pastad                                    # Server
    make pasta                                     # Client
    make                                           # all
	make static                                    # static binaries

Create a `pastad.toml` file using the provided example (`pastad.toml.example`) and run the server with

    ./pastad

### environment variables

In addition to the config file, `pastad` can also be configured via environmental variables. This might be useful for running pasta as a container without a dedicated config file. Supported environmental variables are:

| Key | Description |
|-----|-------------|
| `PASTA_BASEURL` | Base URL for the pasta instance |
| `PASTA_PASTADIR` | Data directory for pastas |
| `PASTA_BINDADDR` | Address to bind the server to |
| `PASTA_MAXSIZE` | Maximum size (in Bytes) for new pastas |
| `PASTA_CHARACTERS` | Number of characters for new pastas |
| `PASTA_MIMEFILE` | MIME file |
| `PASTA_EXPIRE` | Default expiration time (in seconds) |
| `PASTA_CLEANUP` | Seconds between cleanup cycles |
| `PASTA_REQUESTDELAY` | Delay between requests from the same host in milliseconds |

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

`pasta` is the CLI utility for making the creation of a pasta as easy as possible.  
For instance, if you want to push the `README.md` file and create a pasta out of it:

    pasta README.md
    pasta -r http://localhost:8199 REAME.md          # Define a custom remote server

`pasta` reads the config from `~/.pasta.toml` (see the [example file](pasta.toml.example))

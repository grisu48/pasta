![Build status badge](https://github.com/grisu48/pasta/workflows/pastad/badge.svg)

# pasta

Stupid simple pastebin service written in go.

The aim of this project is to create a simple pastebin service for self-hosting. pasta is self-contained, this means it does not need any additional services, e.g. a database to function. All it needs is a data directory and a config `toml` file and it will work.

This README contains the most important information. See the [docs](docs/index.md) folder for more documentation, e.g. the [getting-started](docs/getting-started.md) guide.

## Run as container (podman/docker)

The easiest way of self-hosting a `pasta` server is via the provided container from `ghcr.io/grisu48/pasta:latest`. Setup your own `pasta` instance is as easy as:

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

The container runs fine as rootless container (podman).

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
| `PASTA_PUBLICPASTAS` | Number of public pastas to be displayed |

### macros

The `BASEURL` setting, defined either via configuration file or via the `PASTA_BASEURL` environment variable, supports custom macros, that should help you in various scenarios. Macros are pre-defined strings, which will be replaced.

The following macros are currently supported

| Macro | Replaced with | Example |
| `$hostname` | Local hostname | `localhost` |

A usage example would be to e.g. define the following in your local `pastad.conf`

```toml
BaseURL = "http://$hostname:8199"    # base URL as used within pasta
```

# Usage

Assuing the server runs on http://localhost:8199, you can use the `pasta` CLI tool (See below) or `curl`:

    curl -X POST 'http://localhost:8199' --data-binary @README.md

## pasta CLI

`pasta` is the CLI utility for making the creation of a pastas (i.e. files submitted to a pasta server) as easy as possible.  
For instance, if you want to push the `README.md` file and create a pasta out of it:

    pasta README.md
    pasta -r http://localhost:8199 REAME.md          # Define a custom remote server

`pasta` reads the config from `~/.pasta.toml` (see the [example file](pasta.toml.example))

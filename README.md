[![Build Status](https://travis-ci.org/grisu48/pasta.svg?branch=main)](https://travis-ci.org/grisu48/pasta)

# pasta

Stupid simple pastebin service written in go

## Build and run

    make pastad                                    # Server
    make pasta                                     # Client
    make                                           # all

Then create a `pastad.toml` file using the provided example (`pastad.toml.example`) and run the server with

    ./pastad

### Docker

    make docker

Or manually:

    docker build . -t feldspaten.org/pasta         # Build docker container

Create or run the container with

    docker container create --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta
    docker container run --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta

The container needs a `data` directory with a valid `pastad.toml` (See the [example file](pastad.toml.example), otherwise default values will be used).

# Usage

Assuing the server runs on http://localhost:8199, you can use the `pasta` tool or simply `curl` :

    curl -X POST 'http://localhost:8199' -d @README.md

## pasta CLI

`pasta` is the CLI utility for easy handling

    pasta < README.md
    pasta -r http://localhost:8199

`pasta` reads the `~/.pasta.toml` file (see the [example file](pasta.toml.example))
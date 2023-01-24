# Building

## Build and run from source

Use the provided `Makefile` commands:

    make                                           # all

    make pastad                                    # Server
    make pasta                                     # Client

	make static                                    # build static binaries (for release)

## Build docker image

    make docker

Or manually:

    docker build . -t feldspaten.org/pasta         # Build docker container

Create or run the container with

    docker container create --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta
    docker container run --name pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data feldspaten.org/pasta

The container needs a `data` directory with a valid `pastad.toml` (See the [example file](pastad.toml.example), otherwise default values will be used).
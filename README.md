# pasta

Stupid simple pastebin service written in go

## Build

    make pastad                                    # Server
    make pasta                                     # Client
    make                                           # all

### Docker

    docker build . -t feldspaten.org/pasta         # Build docker container

Run the container with

    docker container create --name pasta feldspaten.org/pasta -p 8199:8199 -v ABSOLUTE_PATH_TO_DATA_DIR:/data

# Todolist

* Spam-protection (max 5 requests per minute per IP)
* Paste expire

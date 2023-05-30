default: all
all: pasta pastad
static: pasta-static pastad-static

.PHONY: all test clean

requirements:
	go get github.com/BurntSushi/toml
	go get github.com/akamensky/argparse
	
pasta: cmd/pasta/*.go
	go build -o pasta $^
pastad: cmd/pastad/*.go
	go build -o pastad $^
pasta-static: cmd/pasta/*.go
	CGO_ENABLED=0 go build -ldflags="-w -s" -o pasta $^
pastad-static: cmd/pastad/*.go
	CGO_ENABLED=0 go build -ldflags="-w -s" -o pastad $^

test: pastad pasta
	go test ./...
	# TODO: This syntax is horrible :-)
	bash -c 'cd test && ./test.sh'

container-docker: Containerfile pasta pastad
	docker build . -t feldspaten.org/pasta

container-podman: Containerfile pasta pastad
	podman build . -t feldspaten.org/pasta


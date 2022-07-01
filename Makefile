default: all
all: pasta pastad
static: pasta-static pastad-static

.PHONY: all test clean

requirements:
	go get github.com/BurntSushi/toml
	go get github.com/akamensky/argparse
	
pasta: cmd/pasta/pasta.go cmd/pasta/storage.go
	go build $^
pastad: cmd/pastad/pastad.go cmd/pastad/storage.go
	go build $^
pasta-static: cmd/pasta/pasta.go cmd/pasta/storage.go
	CGO_ENABLED=0 go build -ldflags="-w -s" -o pasta $^
pastad-static: cmd/pastad/pastad.go cmd/pastad/storage.go
	CGO_ENABLED=0 go build -ldflags="-w -s" -o pastad $^

test: pastad pasta
	go test ./...
	# TODO: This syntax is horrible :-)
	bash -c 'cd test && ./test.sh'

container-docker: Dockerfile pasta pastad
	docker build . -t feldspaten.org/pasta

container-podman: Dockerfile pasta pastad
	podman build . -t feldspaten.org/pasta

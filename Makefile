default: all
all: pasta pastad

requirements:
	go get github.com/BurntSushi/toml
	
pasta: cmd/pasta/pasta.go cmd/pasta/storage.go
	go build $^
pastad: cmd/pastad/pastad.go cmd/pastad/storage.go
	go build $^

test: pastad
	go test ./...
	# TODO: This syntax is horrible :-)
	bash -c 'cd test && ./test.sh'

docker: Dockerfile pasta pastad
	docker build . -t feldspaten.org/pasta

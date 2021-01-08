default: all
all: pasta pastad

.PHONY: all test clean

requirements:
	go get github.com/BurntSushi/toml
	
pasta: cmd/pasta/pasta.go cmd/pasta/storage.go
	go build $^
pastad: cmd/pastad/pastad.go cmd/pastad/storage.go
	go build $^

test: pastad pasta
	go test ./...
	# TODO: This syntax is horrible :-)
	bash -c 'cd test && ./test.sh'

docker: Dockerfile pasta pastad
	docker build . -t feldspaten.org/pasta

deploy: Dockerfile pasta pastad
	docker build . -t grisu48/pasta
	docker push grisu48/pasta

default: all
all: pasta pastad

requirements:
	go get github.com/mattn/go-sqlite3
	go get github.com/BurntSushi/toml
	
pasta: cmd/pasta/*.go
	go build $^
pastad: cmd/pastad/*.go
	go build $^

test:
	go test ./...
	./test.sh

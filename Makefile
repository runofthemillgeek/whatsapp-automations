start: install-deps
	go run cmd/main.go

install-deps:
	go install ./...

build:
	# CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o out/wa-autoresponder.linux-amd64 cmd/main.go
	CC=x86_64-linux-musl-gcc CXX=x86_64-linux-musl-g++ GOARCH=amd64 GOOS=linux CGO_ENABLED=1 go build -ldflags "-linkmode external -extldflags -static" -o out/wa-autoresponder.linux-amd64 cmd/main.go
	# docker run --rm -v $(PWD):/go/src/app -w /go/src/app -e CGO_ENABLED=1 -e GOOS=linux -e GOARCH=amd64 golang:latest go build -o out/wa-autoresponder.linux-amd64 cmd/main.go

deploy: build
	ssh sangeeth@athena.home "bash -l -c 'mkdir -p ~/.local/data/wa-autoresponder'"
	ssh sangeeth@athena.home "bash -l -c 'systemctl --user stop wa-autoresponder || true'"
	rsync -avhP out/wa-autoresponder.linux-amd64 athena.home:.local/bin/wa-autoresponder
	ssh sangeeth@athena.home "bash -l -c 'systemctl --user start wa-autoresponder'"

.PHONY: start install-deps build
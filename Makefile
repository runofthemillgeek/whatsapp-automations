start: install-deps
	go run cmd/main.go

install-deps:
	go install ./...

.PHONY: start install-deps
install:
	go install ./cmd/mct

lint:
	golangci-lint run ./...

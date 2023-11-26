set dotenv-load

go := env("GO", "go")

lint:
	golangci-lint run ./...

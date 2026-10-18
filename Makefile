REVIVE_VERSION := v1.17.1

.PHONY: test lint build run-dev

test:
	go test ./...

lint:
	@test -z "$$(gofmt -l .)" || { echo "not gofmt'ed:"; gofmt -l .; exit 1; }
	go vet ./...
	go run github.com/mgechev/revive@$(REVIVE_VERSION) -config revive.toml -set_exit_status ./...

build:
	go build -o tool ./cmd/tool/...

run-dev:
	@for f in ./testdata/valid/*.yaml; do go run ./cmd/tool lint "$$f" || exit 1; done

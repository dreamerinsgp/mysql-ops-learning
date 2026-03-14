.PHONY: build test smoke clean

BINARY := mysql-ops

build:
	go build -o $(BINARY) ./cmd/main.go

test:
	go test -v -count=1 ./...

# 需要 MYSQL_DSN，本地可执行 make smoke
smoke: build
	./scripts/smoke_test.sh

clean:
	rm -f $(BINARY)

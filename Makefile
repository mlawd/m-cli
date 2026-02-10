APP_NAME := m
ARGS ?=

.PHONY: run build install clean

run:
	go run ./cmd/m $(ARGS)

build:
	go build -o ./bin/$(APP_NAME) ./cmd/m

install:
	go install ./cmd/m

clean:
	rm -rf ./bin

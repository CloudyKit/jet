run:
	go run main.go

deps:
	go get -u github.com/shurcooL/vfsgen

build:
	go generate
	go build -tags=deploy_build -o bin/app main.go

.PHONY: run build deps

APP=buddyflix
VERSION=0.1.0

.PHONY: build armhf amd64 clean
build: amd64 armhf

amd64:
	mkdir -p dist/amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o dist/amd64/$(APP) ./cmd/buddyflix
	cp -r web dist/amd64/

armhf:
	mkdir -p dist/armhf
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -trimpath -ldflags="-s -w" -o dist/armhf/$(APP) ./cmd/buddyflix
	cp -r web dist/armhf/

clean:
	rm -rf dist

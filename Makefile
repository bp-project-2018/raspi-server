build:
	cd cmd/server && go build

run: build
	cd cmd/server && ./server
	
test:
	cd internal/server && go test

upload:
	cd cmd/server && env GOOS=linux GOARCH=arm go build && rsync -rt server pi@kronos.local: && rsync --exclude='.git' -rt static pi@kronos.local: && rsync --exclude='.git' -rt config pi@kronos.local:
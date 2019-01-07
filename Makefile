build:
	cd cmd/protoclient && go build

run: build
	./cmd/protoclient/protoclient
	
test:
	cd internal/commproto && go test

upload:
	cd cmd/protoclient && env GOOS=linux GOARCH=arm go build && rsync -rtv protoclient pi@kronos.local:
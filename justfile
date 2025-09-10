alias b := build

build:
	go build -o andurel-dev main.go

move:
	sudo mv andurel-dev /usr/local/bin/andurel-dev

full:
	just build
	just move

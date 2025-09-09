alias b := build

build:
	go build -o andurel main.go

move:
	sudo mv andurel /usr/local/bin/andurel

full:
	just build
	just move

alias b := build

build:
	go build -o andurel-dev main.go

move:
	mv andurel-dev ../

scaf-sqlite:
	cd ../ && ./andurel-dev new myp-sqlite -d sqlite && mv ./andurel-dev ./myp-sqlite && cd ./myp-sqlite && cp .env.example .env && just new-migration users

scaf-psql:
	cd ../ && ./andurel-dev new myp-psql && mv ./andurel-dev ./myp-psql && cd ./myp-psql && cp .env.example .env && just new-migration users

full-sqlite:
	just build
	just move
	just scaf-sqlite

full-psql:
	just build
	just move
	just scaf-psql

full:
	just build
	just move

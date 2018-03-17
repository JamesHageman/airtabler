all:
	go build

race:
	go build -race

run: all
	./airtabler

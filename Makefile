all:
	go build -race

run: all
	./airtabler

all:
	go build -race

run: all
	./airtabler

loadtest: all
	sh loadtest.sh

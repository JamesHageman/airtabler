all:
	go build -race -v

install:
	go install -v

run: all
	./airtabler

loadtest: all
	sh loadtest.sh

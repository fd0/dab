SRC=$(wildcard *.go)

.PHONY: all clean

all: dab

dab: $(SRC)
	GOARCH=386 go build -ldflags '-s -w' -o $@ .

clean:
	rm -f dab

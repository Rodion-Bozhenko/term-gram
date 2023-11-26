OPENSSL_PATH=$(shell brew --prefix openssl)

CGO_CFLAGS=-I/usr/local/include
CGO_LDFLAGS=-L/usr/local/lib -L$(OPENSSL_PATH)/lib -ltdjson -Wl,-rpath,./td/build

build:
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go build .

run:
	CGO_CFLAGS="$(CGO_CFLAGS)" CGO_LDFLAGS="$(CGO_LDFLAGS)" go run .

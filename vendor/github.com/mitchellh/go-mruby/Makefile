MRUBY_COMMIT ?= 1.2.0

all: libmruby.a test

clean:
	rm -rf vendor
	rm -f libmruby.a

gofmt:
	@echo "Checking code with gofmt.."
	gofmt -s *.go >/dev/null

lint:
	sh golint.sh

libmruby.a: vendor/mruby
	cd vendor/mruby && ${MAKE}
	cp vendor/mruby/build/host/lib/libmruby.a .

vendor/mruby:
	mkdir -p vendor
	git clone https://github.com/mruby/mruby.git vendor/mruby
	cd vendor/mruby && git reset --hard && git clean -fdx
	cd vendor/mruby && git checkout ${MRUBY_COMMIT}

test: gofmt lint
	go test -v

.PHONY: all clean libmruby.a test lint

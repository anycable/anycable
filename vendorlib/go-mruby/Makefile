MRUBY_COMMIT ?= 1.2.0
MRUBY_VENDOR_DIR ?= mruby-build

all: libmruby.a test

clean:
	rm -rf ${MRUBY_VENDOR_DIR}
	rm -f libmruby.a

gofmt:
	@echo "Checking code with gofmt.."
	gofmt -s *.go >/dev/null

lint:
	GO111MODULE=off go get golang.org/x/lint/golint
	golint ./...

staticcheck:
	GO111MODULE=off go get honnef.co/go/tools/cmd/staticcheck
	staticcheck ./...

libmruby.a: ${MRUBY_VENDOR_DIR}/mruby
	cd ${MRUBY_VENDOR_DIR}/mruby && ${MAKE}
	cp ${MRUBY_VENDOR_DIR}/mruby/build/host/lib/libmruby.a .

${MRUBY_VENDOR_DIR}/mruby:
	mkdir -p ${MRUBY_VENDOR_DIR}
	git clone https://github.com/mruby/mruby.git ${MRUBY_VENDOR_DIR}/mruby
	cd ${MRUBY_VENDOR_DIR}/mruby && git reset --hard && git clean -fdx
	cd ${MRUBY_VENDOR_DIR}/mruby && git checkout ${MRUBY_COMMIT}

test: libmruby.a gofmt lint
	go test -v

.PHONY: all clean libmruby.a test lint staticcheck

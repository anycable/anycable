module github.com/anycable/anycable-go

go 1.23.0

toolchain go1.23.1

require (
	github.com/FZambia/sentinel v1.1.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fullstorydev/grpchan v1.1.1
	github.com/go-chi/chi/v5 v5.2.1
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/gomodule/redigo v1.9.2
	github.com/google/gops v0.3.28
	github.com/gorilla/websocket v1.5.3
	github.com/hofstadter-io/cinful v1.0.0
	github.com/joomcode/errorx v1.2.0
	github.com/lmittmann/tint v1.0.7
	github.com/matoous/go-nanoid v1.5.1
	github.com/mattn/go-isatty v0.0.20
	github.com/mitchellh/go-mruby v0.0.0-20200315023956-207cedc21542
	github.com/nats-io/nats.go v1.41.1
	github.com/posthog/posthog-go v1.4.7
	github.com/redis/rueidis v1.0.57
	github.com/smira/go-statsd v1.3.4
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli/v2 v2.27.6
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0
	golang.org/x/net v0.39.0
	google.golang.org/grpc v1.71.1
)

// use vendored go-mruby
replace github.com/mitchellh/go-mruby => ./vendorlib/go-mruby

require (
	github.com/BurntSushi/toml v1.5.0
	github.com/golang-jwt/jwt/v5 v5.2.2
	github.com/sony/gobreaker v1.0.0
)

require (
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/google/go-tpm v0.9.3 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.4 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250414145226-207652e42e2e // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/jhump/protoreflect v1.17.0 // indirect
	github.com/nats-io/nats-server/v2 v2.11.1
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

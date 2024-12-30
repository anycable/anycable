module github.com/anycable/anycable-go

go 1.23

require (
	github.com/FZambia/sentinel v1.1.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fullstorydev/grpchan v1.1.1
	github.com/go-chi/chi/v5 v5.2.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/gomodule/redigo v1.9.2
	github.com/google/gops v0.3.28
	github.com/gorilla/websocket v1.5.3
	github.com/hofstadter-io/cinful v1.0.0
	github.com/joomcode/errorx v1.2.0
	github.com/lmittmann/tint v1.0.6
	github.com/matoous/go-nanoid v1.5.1
	github.com/mattn/go-isatty v0.0.20
	github.com/mitchellh/go-mruby v0.0.0-20200315023956-207cedc21542
	github.com/nats-io/nats.go v1.38.0
	github.com/posthog/posthog-go v1.2.24
	github.com/redis/rueidis v1.0.51
	github.com/smira/go-statsd v1.3.4
	github.com/stretchr/testify v1.10.0
	github.com/urfave/cli/v2 v2.27.5
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/exp v0.0.0-20241217172543-b2144cdd0a67
	golang.org/x/net v0.32.0
	google.golang.org/grpc v1.69.0
)

// use vendored go-mruby
replace github.com/mitchellh/go-mruby => ./vendorlib/go-mruby

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/sony/gobreaker v1.0.0
)

require (
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.3 // indirect
	golang.org/x/time v0.8.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241216192217-9240e9c98484 // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.6 // indirect
	github.com/jhump/protoreflect v1.17.0 // indirect
	github.com/nats-io/nats-server/v2 v2.10.24
	github.com/nats-io/nkeys v0.4.9 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/crypto v0.31.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/protobuf v1.36.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

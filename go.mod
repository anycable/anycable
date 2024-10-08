module github.com/anycable/anycable-go

go 1.23

require (
	github.com/FZambia/sentinel v1.1.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fullstorydev/grpchan v1.1.1
	github.com/go-chi/chi/v5 v5.1.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.4
	github.com/gomodule/redigo v1.9.2
	github.com/google/gops v0.3.28
	github.com/gorilla/websocket v1.5.3
	github.com/hofstadter-io/cinful v1.0.0
	github.com/joomcode/errorx v1.1.1
	github.com/lmittmann/tint v1.0.5
	github.com/matoous/go-nanoid v1.5.0
	github.com/mattn/go-isatty v0.0.20
	github.com/mitchellh/go-mruby v0.0.0-20200315023956-207cedc21542
	github.com/nats-io/nats.go v1.37.0
	github.com/posthog/posthog-go v1.2.24
	github.com/redis/rueidis v1.0.47
	github.com/smira/go-statsd v1.3.3
	github.com/stretchr/testify v1.9.0
	github.com/urfave/cli/v2 v2.27.4
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/exp v0.0.0-20241004190924-225e2abe05e6
	golang.org/x/net v0.30.0
	google.golang.org/grpc v1.67.1
)

// use vendored go-mruby
replace github.com/mitchellh/go-mruby => ./vendorlib/go-mruby

require github.com/sony/gobreaker v1.0.0

require (
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/bufbuild/protocompile v0.14.1 // indirect
	github.com/klauspost/compress v1.17.10 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/minio/highwayhash v1.0.3 // indirect
	github.com/nats-io/jwt/v2 v2.7.0 // indirect
	golang.org/x/time v0.7.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240930140551-af27646dc61f // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.5 // indirect
	github.com/jhump/protoreflect v1.17.0 // indirect
	github.com/nats-io/nats-server/v2 v2.10.21
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/xrash/smetrics v0.0.0-20240521201337-686a1a2994c1 // indirect
	golang.org/x/crypto v0.28.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

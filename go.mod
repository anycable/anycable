module github.com/anycable/anycable-go

go 1.27.0

toolchain go1.27.3

// use vendored go-mruby
replace github.com/mitchellh/go-mruby => ./vendorlib/go-mruby

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/durable-streams/durable-streams/packages/client-go v0.1.0
	github.com/fullstorydev/grpchan v1.1.2
	github.com/go-chi/chi/v5 v5.5.0
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/google/gops v0.3.29
	github.com/gorilla/websocket v1.5.3
	github.com/hofstadter-io/cinful v1.0.0
	github.com/joomcode/errorx v1.2.0
	github.com/lmittmann/tint v1.2.1
	github.com/matoous/go-nanoid v1.5.1
	github.com/mattn/go-isatty v0.0.24
	github.com/mitchellh/go-mruby v0.0.0-20200315023956-207cedc21542
	github.com/nats-io/nats-server/v2 v2.15.1
	github.com/nats-io/nats.go v1.54.0
	github.com/posthog/posthog-go v1.35.0
	github.com/pusher/pusher-http-go/v5 v5.1.1
	github.com/redis/rueidis v1.0.79
	github.com/smira/go-statsd v1.3.4
	github.com/sony/gobreaker v1.0.0
	github.com/stretchr/testify v1.12.1
	github.com/urfave/cli/v2 v2.27.7
	go.uber.org/automaxprocs v1.6.0
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e
	golang.org/x/net v0.61.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/andybalholm/brotli v1.2.6 // indirect
	github.com/antithesishq/antithesis-sdk-go v0.8.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/goccy/go-json v0.11.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/jhump/protoreflect v1.19.0 // indirect
	github.com/jhump/protoreflect/v2 v2.0.0 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/minio/highwayhash v1.0.4 // indirect
	github.com/nats-io/jwt/v2 v2.8.2 // indirect
	github.com/nats-io/nkeys v0.4.16 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.8.0 // indirect
	github.com/xrash/smetrics v0.0.0-20261013231157-39c47d93efba // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261012220534-4341edf3f941 // indirect
)

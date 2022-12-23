module github.com/anycable/anycable-go

go 1.20

require (
	github.com/FZambia/sentinel v1.1.0
	github.com/apex/log v1.9.0
	github.com/go-chi/chi/v5 v5.0.8
	github.com/golang-collections/go-datastructures v0.0.0-20150211160725-59788d5eb259
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/protobuf v1.5.2
	github.com/gomodule/redigo v1.8.8
	github.com/google/gops v0.3.23
	github.com/gorilla/websocket v1.5.0
	github.com/joomcode/errorx v1.1.0
	github.com/matoous/go-nanoid v1.5.0
	github.com/mattn/go-isatty v0.0.14
	github.com/mitchellh/go-mruby v0.0.0-20200315023956-207cedc21542
	github.com/namsral/flag v1.7.4-pre
	github.com/nats-io/nats.go v1.24.0
	github.com/smira/go-statsd v1.3.2
	github.com/rueian/rueidis v0.0.90
	github.com/stretchr/testify v1.8.2
	github.com/syossan27/tebata v0.0.0-20180602121909-b283fe4bc5ba
	github.com/urfave/cli/v2 v2.11.1
	go.uber.org/automaxprocs v1.5.1
	golang.org/x/net v0.7.0
	google.golang.org/grpc v1.53.0
)

// https://github.com/stretchr/testify/pull/1229
replace github.com/stretchr/testify => github.com/palkan/testify v0.0.0-20220714120938-9ebebef47942

require (
	github.com/klauspost/compress v1.16.0 // indirect
	github.com/minio/highwayhash v1.0.2 // indirect
	github.com/nats-io/jwt/v2 v2.3.0 // indirect
	golang.org/x/time v0.3.0 // indirect
)

require (
	github.com/cpuguy83/go-md2man/v2 v2.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/nats-io/nats-server/v2 v2.9.14
	github.com/nats-io/nkeys v0.3.0 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/xrash/smetrics v0.0.0-20201216005158-039620a65673 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	google.golang.org/genproto v0.0.0-20230216225411-c8e22ba71e44 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

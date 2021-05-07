sudo apt-get install bison redis-server -yq --no-install-recommends
redis-server --daemonize yes
make prepare prepare-mruby
env GO111MODULE=off go get github.com/anycable/websocket-bench
env GO111MODULE=off go get github.com/google/gops
env GO111MODULE=off go get github.com/evilmartians/lefthook

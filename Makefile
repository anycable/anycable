all: build

build:
	protoc --ruby_out=./lib/anycable/rpc --grpc_out=./lib/anycable/rpc --proto_path=./protos --plugin=protoc-gen-grpc=`which grpc_tools_ruby_protoc_plugin.rb` ./protos/rpc.proto

all: build

gem/grpc-tools:
	@(gem info -l grpc-tools | grep grpc-tools) > /dev/null || gem install grpc-tools

build: gem/grpc-tools
	grpc_tools_ruby_protoc -I ./protos --ruby_out=./lib/anycable/protos --grpc_out=./lib/anycable/grpc ./protos/rpc.proto
	sed -i'' '/'rpc_pb'/d' ./lib/anycable/grpc/rpc_services_pb.rb
	sed -i'' 's/module RPC/module GRPC/g' ./lib/anycable/grpc/rpc_services_pb.rb
	sed -i'' 's/Anycable/AnyCable/g' ./lib/anycable/protos/*_pb.rb
	sed -i'' 's/Anycable/AnyCable/g' ./lib/anycable/grpc/*_pb.rb
	bundle exec rubocop -A ./lib/anycable/protos ./lib/anycable/grpc

release:
	gem release anycable-core
	gem release anycable -t
	git push
	git push --tags

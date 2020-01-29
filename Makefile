all: build

build:
	grpc_tools_ruby_protoc -I ./protos --ruby_out=./lib/anycable/rpc --grpc_out=./lib/anycable/rpc ./protos/rpc.proto
	sed -i '' '/'rpc_pb'/d' ./lib/anycable/rpc/rpc_services_pb.rb
	sed -i '' 's/Anycable/AnyCable/g' ./lib/anycable/rpc/*_pb.rb
	bundle exec rubocop -a ./lib/anycable/rpc

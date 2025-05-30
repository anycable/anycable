package rpc

import (
	"strings"

	"google.golang.org/grpc/resolver"
)

const multiHostGRPCSchema = "grpc-list"

type multiHostGRPCResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
}

func (r *multiHostGRPCResolver) start() error {
	addrStrs := strings.Split(r.target.URL.Host, ",")
	endpoints := make([]resolver.Endpoint, len(addrStrs))
	for i, s := range addrStrs {
		addr := resolver.Address{Addr: s}
		endpoints[i] = resolver.Endpoint{Addresses: []resolver.Address{addr}}
	}
	return r.cc.UpdateState(resolver.State{Endpoints: endpoints})
}
func (*multiHostGRPCResolver) ResolveNow(resolver.ResolveNowOptions) {}
func (*multiHostGRPCResolver) Close()                                {}

type multiHostGRPCResolverBuilder struct{}

func (*multiHostGRPCResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	r := &multiHostGRPCResolver{
		target: target,
		cc:     cc,
	}
	err := r.start()
	return r, err
}
func (*multiHostGRPCResolverBuilder) Scheme() string { return multiHostGRPCSchema }

func init() {
	resolver.Register(&multiHostGRPCResolverBuilder{})
}

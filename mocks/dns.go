package mocks

import (
	context "context"
	"net"
	"time"

	"github.com/miekg/dns"
)

func MockDNSServer(zone string, txt []string) func() {
	dns.HandleFunc(zone, func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		for _, txt := range txt {
			m.Answer = append(m.Answer, &dns.TXT{
				Hdr: dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
				Txt: []string{txt},
			})
		}
		w.WriteMsg(m) // nolint:errcheck
	})

	wait := make(chan struct{})

	server := &dns.Server{Addr: ":0", Net: "udp"}
	server.NotifyStartedFunc = func() {
		close(wait)
	}

	go server.ListenAndServe() // nolint:errcheck

	<-wait

	addr := server.PacketConn.LocalAddr().String()

	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * 500,
			}
			return d.DialContext(ctx, "udp", addr)
		},
	}

	return func() {
		net.DefaultResolver.PreferGo = false
		net.DefaultResolver.Dial = nil
		server.Shutdown() // nolint:errcheck
	}
}

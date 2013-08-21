package main

import (
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

var registeredNames map[string]net.IP

func registerLocal(w dns.ResponseWriter, req *dns.Msg) {
	log.Printf("Trying to register %v\n", req.Question[0].Name)

	m := new(dns.Msg)
	m.SetReply(req)

	var a net.IP
	pieces := dns.SplitDomainName(req.Question[0].Name)
	host := pieces[len(pieces)-3]

	if ip, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		a = ip.IP
		registeredNames[host] = a
		log.Printf("TCPAddr registeredNames[%s] => %v\n", host, a)
	} else if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		a = ip.IP
		registeredNames[host] = a
		log.Printf("UDPAddr registeredNames[%s] => %v\n", host, a)
	} else {
		registeredNames[host] = net.ParseIP("1.2.3.4")
		log.Println("Failed to find address")
		log.Printf("Setting anyway: registeredNames[%s] => %v\n", host, a)
	}

	if a != nil {
		var rr dns.RR
		rr = new(dns.A)
		rr.(*dns.A).Hdr = dns.RR_Header{
			Name: req.Question[0].Name,
			Rrtype: dns.TypeA,
			Class: dns.ClassINET,
			Ttl: 60,
		}
		rr.(*dns.A).A = a.To4()
		m.Extra = append(m.Extra, rr)
	}

	w.WriteMsg(m)
}

func handleLocal(w dns.ResponseWriter, req *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(req)

	if req.Question[0].Qtype == dns.TypeA {
		pieces := dns.SplitDomainName(req.Question[0].Name)
		if len(pieces) >= 2 {
			if ip, found := registeredNames[pieces[len(pieces)-2]]; found {
				var rr dns.RR
				rr = new(dns.A)
				rr.(*dns.A).Hdr = dns.RR_Header{
					Name: req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class: dns.ClassINET,
					Ttl: 60,
				}
				rr.(*dns.A).A = ip
				m.Answer = append(m.Answer, rr)
			}
		}
	}

	w.WriteMsg(m)
}

func serve(proto string, port int) {
	log.Printf("Attempting to serve on port %d over %s\n", port, proto)
	err := dns.ListenAndServe(fmt.Sprintf(":%d", port), proto, nil)
	if err != nil {
		log.Fatal("Failed to set up %v server: %v\n", proto, err.Error())
	}
}

func main() {
	registeredNames = make(map[string]net.IP)

	port := flag.Int("port", 9753, "Port to serve from")
	registerAt := flag.String("register", "in", "Host at which to register")
	flag.Parse()

	regName := fmt.Sprintf("%s.4m.", *registerAt)

	log.Printf("Serving on port %d\n", *port)
	log.Printf("Register hosts at (whatever).%s\n", regName)

	dns.HandleFunc(regName, registerLocal)
	dns.HandleFunc("4m.", handleLocal)
	go serve("tcp4", *port)
	go serve("udp4", *port)

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

infiniteloop:
	for {
		select {
		case s := <-sig:
			fmt.Printf("Signal (%d) received, stopping\n", s)
			break infiniteloop
		}
	}
}

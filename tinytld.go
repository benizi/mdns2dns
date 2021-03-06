package main

import (
	"encoding/gob"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"
)

type Registration struct {
   IP net.IP
   Registered time.Time
}

var registeredNames map[string]Registration
var tld, registration, hostsFile string

func registerName(name string, ip net.IP) {
	registeredNames[name] = Registration{
		IP: ip,
		Registered: time.Now(),
	}
	saveHostsFile()
}

func registerLocal(w dns.ResponseWriter, req *dns.Msg) {
	log.Printf("Trying to register %v\n", req.Question[0].Name)

	m := new(dns.Msg)
	m.SetReply(req)

	var a net.IP
	pieces := dns.SplitDomainName(req.Question[0].Name)

	if len(pieces) < 3 {
		// tried to set the root name - write empty reply and return
		w.WriteMsg(m)
		return
	}

	host := pieces[len(pieces)-3]

	if ip, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		a = ip.IP
		registerName(host, a)
		log.Printf("TCPAddr registeredNames[%s] => %v\n", host, a)
	} else if ip, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		a = ip.IP
		registerName(host, a)
		log.Printf("UDPAddr registeredNames[%s] => %v\n", host, a)
	} else {
		registerName(host, net.ParseIP("1.2.3.4"))
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
			if entry, found := registeredNames[pieces[len(pieces)-2]]; found {
				var rr dns.RR
				rr = new(dns.A)
				rr.(*dns.A).Hdr = dns.RR_Header{
					Name: req.Question[0].Name,
					Rrtype: dns.TypeA,
					Class: dns.ClassINET,
					Ttl: 60,
				}
				rr.(*dns.A).A = entry.IP
				m.Answer = append(m.Answer, rr)
			}
		}
	}

	w.WriteMsg(m)
}

func httpRemoteIP(r *http.Request) string {
	for _, header := range([]string{"X-Real-Ip", "X-Forwarded-For"}) {
		addr, found := r.Header[header]
		if found {
			return addr[0]
		}
	}

	addr, err := net.ResolveTCPAddr("tcp4", r.RemoteAddr)
	if err == nil {
		return addr.IP.String()
	}
	return "1.2.3.4"
}

func handleHttpListing(w http.ResponseWriter, r *http.Request) {
	log.Println("Serving host listing")
	fmt.Fprintf(w, "<html><head><title>Currently registered hosts</title></head><body>\n")
	fmt.Fprintf(w, "<form action=\"/register\" method=\"POST\">")
	fmt.Fprintf(w, "<input name=\"name\" type=\"text\" placeholder=\"new host\"/>")
	fmt.Fprintf(w, "<input name=\"ip\" type=\"text\" value=\"%s\"/>", httpRemoteIP(r))
	fmt.Fprintf(w, "<input name=\"submit\" type=\"submit\" value=\"register\"/>")
	fmt.Fprintf(w, "</form>")

	if len(registeredNames) == 0 {
		fmt.Fprintf(w, "<p>No registered names</p>")
	} else {
		var names []string
		for k := range(registeredNames) {
			names = append(names, k)
		}
		sort.Strings(names)
		fmt.Fprintf(w, "<table><thead><tr><th>Name</th><th>IP</th><th>Time</th></thead><tbody>")
		rowTemplate := template.New("list row")
		rowTemplate, err := rowTemplate.Parse("<tr><td>{{.Name}}</td><td>{{.IP}}</td><td>{{.Registered}}</td></tr>")
		if err == nil {
			for _, name := range(names) {
				if entry, ok := registeredNames[name]; ok {
					data := struct {
						Name string
						IP string
						Registered string
					}{
						name,
						entry.IP.String(),
						entry.Registered.String(),
					}
					rowTemplate.Execute(w, data)
				}
			}
		}
		fmt.Fprintf(w, "</tbody></table>")
	}
	fmt.Fprintf(w, "</body>\n")
}

func handleHttpRegistration(w http.ResponseWriter, r *http.Request) {
	log.Println("Attempting HTTP registration")
	err := r.ParseForm()
	if err == nil {
		name, foundName := r.Form["name"]
		if foundName && len(name) == 0 {
			foundName = false
		}
		ipText, foundIP := r.Form["ip"]
		if foundIP && len(ipText) == 0 {
			foundIP = false
		}
		if foundName && foundIP {
			ip := net.ParseIP(ipText[0])
			if err == nil {
				registerName(name[0], ip)
				log.Printf("Success: %s => %v\n", name, ip)
			} else {
				fmt.Fprintf(w, "Bad IP? %v (err: %s)\n", ipText, err.Error())
			}
		} else {
			if !foundName {
				fmt.Fprintf(w, "Missing name\n")
			}
			if !foundIP {
				fmt.Fprintf(w, "Missing ip\n")
			}
		}
	} else {
		fmt.Fprintf(w, "Fail: %s\n", err.Error())
	}
	http.Redirect(w, r, "/", 303)
}

func serveDNS(proto string, port int) {
	log.Printf("Serving DNS on port %d over %s\n", port, proto)
	err := dns.ListenAndServe(fmt.Sprintf(":%d", port), proto, nil)
	if err != nil {
		log.Fatal("Failed to set up %v server: %v\n", proto, err.Error())
	}
}

func serveHTTP(port int) {
	log.Printf("Serving HTTP on port %d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		log.Fatal("Failed to serve over HTTP: %v\n", err.Error())
	}
}

func saveHostsFile() error {
	log.Println("Saving hosts to", hostsFile)

	file, err := os.OpenFile(hostsFile, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		log.Printf("Failed to save hosts file: %s\n", err.Error())
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)
	encoder.Encode(registeredNames)
	encoder.Encode(tld)
	encoder.Encode(registration)

	log.Println("Saved hosts file")

	return nil
}

func loadHostsFile() error {
	log.Println("Loading hosts from", hostsFile)

	fileinfo, err := os.Stat(hostsFile)
	if err != nil {
		log.Println("Error loading hosts file:", err.Error())
		return err
	}

	if fileinfo.Size() == 0 {
		log.Println("Hosts file is empty")
		return nil
	}

	file, err := os.OpenFile(hostsFile, os.O_RDONLY, 0600)
	if err != nil {
		log.Println("Failed to load hosts file:", err.Error())
		return err
	}

	defer file.Close()

	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&registeredNames)
	if err != nil {
		log.Println("Failed to decode registeredNames:", err.Error())
	}
	err = decoder.Decode(&tld)
	if err != nil {
		log.Println("Failed to decode tld:", err.Error())
	}
	err = decoder.Decode(&registration)
	if err != nil {
		log.Println("Failed to decode registration:", err.Error())
	}

	log.Printf("Loaded %d hosts from %s\n", len(registeredNames), hostsFile)
	return nil
}

func main() {
	registeredNames = make(map[string]Registration)

	tldFlag := flag.String("tld", "host", "TLD to use")
	port := flag.Int("port", 9753, "Port to serve from")
	registerAt := flag.String("register", "in", "Host at which to register")
	httpPort := flag.Int("http", 0, "Port for HTTP serving. 0 = default = none")
	hostsFlag := flag.String("hosts", "saved.hosts", "File for loading/saving hosts")
	flag.Parse()

	tld = fmt.Sprintf("%s.", *tldFlag)
	registration = fmt.Sprintf("%s.%s", *registerAt, tld)

	if len(*hostsFlag) > 0 {
		hostsFile = *hostsFlag
		loadHostsFile()
	}

	log.Printf("Register hosts at (whatever).%s\n", registration)

	dns.HandleFunc(registration, registerLocal)
	dns.HandleFunc(tld, handleLocal)
	go serveDNS("tcp4", *port)
	go serveDNS("udp4", *port)

	if *httpPort != 0 {
		http.HandleFunc("/", handleHttpListing)
		http.HandleFunc("/register", handleHttpRegistration)
		go serveHTTP(*httpPort)
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

infiniteloop:
	for {
		select {
		case s := <-sig:
			fmt.Printf("Signal (%d) received, stopping\n", s)
			saveHostsFile()
			break infiniteloop
		}
	}
}

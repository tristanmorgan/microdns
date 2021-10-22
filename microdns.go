// go build -ldflags "-extldflags '-lm -lstdc++ -static'" .

package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/miekg/dns"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

var ipv4, ipv6, conf, port string
var ttl int
var logflag bool
var mapv4 map[string]string
var mapv6 map[string]string

func main() {
	flag.StringVar(&ipv4, "ipv4", "127.0.0.1", "IPv4 Address")
	flag.StringVar(&ipv6, "ipv6", "::1", "IPv6 Address")
	flag.IntVar(&ttl, "ttl", 86400, "Time to live")
	flag.StringVar(&port, "port", ":8600", "listen port")
	flag.BoolVar(&logflag, "log", false, "Log requests to stdout")
	flag.StringVar(&conf, "conf", "/home/dns.conf", "Config File")
	flag.Parse()
	fmt.Printf("ipv4: %s\n", ipv4)
	fmt.Printf("ipv6: %s\n", ipv6)
	fmt.Printf("ttl : %d\n", ttl)
	fmt.Printf("log : %t\n", logflag)
	fmt.Printf("port: %s\n", port)
	fmt.Printf("conf: %s\n", conf)
	fmt.Println("")
	if _, err := os.Stat(conf); err == nil {
		file, err := os.Open(conf)
		if err != nil {
			fmt.Printf("err : %s\n", err)
		} else {
			defer file.Close()
			mapv4 = make(map[string]string)
			mapv6 = make(map[string]string)
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "#") {
					fields := strings.Fields(line)
					fmt.Printf("line: %q\n", fields)
					if len(fields) == 3 {
						mapv4[fields[0]] = fields[1]
						mapv6[fields[0]] = fields[2]
					}
				}
			}
		}
		fmt.Println("")
	}
	dns.HandleFunc(".", handleRequest)
	go func() {
		srv := &dns.Server{Addr: port, Net: "udp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to set udp listener %s\n", err.Error())
		}
	}()
	go func() {
		srv := &dns.Server{Addr: port, Net: "tcp"}
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal("Failed to set tcp listener %s\n", err.Error())
		}
	}()
	xsig := make(chan os.Signal)
	signal.Notify(xsig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	hsig := make(chan os.Signal)
	signal.Notify(hsig, syscall.SIGHUP)
	for {
		select {
		case s := <-xsig:
			log.Fatalf("\tSignal (%d) received, stopping\n", s)
		case s := <-hsig:
			log.Printf("\tSignal (%d) received, continue\n", s)
		}
	}
}

func handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	domain := r.Question[0].Name
	if logflag {
		ip, _, _ := net.SplitHostPort(w.RemoteAddr().String())
		log.Printf("\t%s\t%s\n", ip, domain)
	}
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	rr1 := new(dns.A)
	rr1.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: uint32(ttl)}
	rr2 := new(dns.AAAA)
	rr2.Hdr = dns.RR_Header{Name: domain, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: uint32(ttl)}
	if val, ok := mapv4[domain]; ok {
		rr1.A = net.ParseIP(val)
		rr2.AAAA = net.ParseIP(mapv6[domain])
	} else {
		rr1.A = net.ParseIP(ipv4)
		rr2.AAAA = net.ParseIP(ipv6)
	}

	if dns.TypeA == r.Question[0].Qtype {
		m.Answer = []dns.RR{rr1}
	} else {
		m.Answer = []dns.RR{rr2}
	}
	w.WriteMsg(m)
}

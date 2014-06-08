package mdnsrpc

import (
	"fmt"
	"net"
	"net/rpc"
	"os"

	"github.com/armon/mdns"
)

func findAll(name string) (results []*mdns.ServiceEntry, err error) {
	results = []*mdns.ServiceEntry{}
	entries := make(chan *mdns.ServiceEntry)
	done := make(chan struct{})
	go func() {
		for entry := range entries {
			results = append(results, entry)
		}
		close(done)
	}()
	mdns.Lookup(fmt.Sprintf("_moxie_%s._tcp", name), entries)
	close(entries)
	<-done
	return
}

/*
LookupOne will use mDNS to lookup exactly one service named _name._tcp and
return a *net/rpc/Client for it.
*/
func LookupOne(name string) (result *rpc.Client, err error) {
	entries, err := findAll(name)
	if err != nil {
		return
	}
	if len(entries) != 1 {
		err = fmt.Errorf("Unable to find exactly one %#v, found %+v", name, entries)
		return
	}
	result, err = rpc.Dial("tcp", fmt.Sprintf("%v:%v", entries[0].Addr.String(), entries[0].Port))
	return
}

/*
LookupAll will use mDNS to lookup all services names _name._tcp and
return *net/rpc.Clients for them.
*/
func LookupAll(name string) (results []*rpc.Client, err error) {
	entries, err := findAll(name)
	if err != nil {
		return
	}
	if len(entries) == 0 {
		err = fmt.Errorf("Unable to find any %#v", name)
		return
	}
	for _, entry := range entries {
		var client *rpc.Client
		if client, err = rpc.Dial("tcp", fmt.Sprintf("%v:%v", entry.Addr.String(), entry.Port)); err == nil {
			results = append(results, client)
		}
	}
	return
}

/*
Publish will serve service using net/rpc on a randomly chosen port on localhost,
publishing it using mDNS as _name_._tcp.
*/
func Publish(name string, service interface{}) (unpublish chan struct{}, err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return
	}

	listenAddr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		err = fmt.Errorf("%v is not a *net.TCPAddr", listener.Addr())
		return
	}

	server := rpc.NewServer()
	server.RegisterName("rpc", service)
	go server.Accept(listener)

	hostname, err := os.Hostname()
	if err != nil {
		return
	}
	entry := &mdns.MDNSService{
		Instance: hostname,
		Service:  fmt.Sprintf("_%s._tcp", name),
		Addr:     listenAddr.IP,
		Port:     listenAddr.Port,
		Info:     name,
	}
	if err = entry.Init(); err != nil {
		return
	}

	mdnsServer, err := mdns.NewServer(&mdns.Config{Zone: entry})
	if err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		<-done
		mdnsServer.Shutdown()
		listener.Close()
	}()

	return
}

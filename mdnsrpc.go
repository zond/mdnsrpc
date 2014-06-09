package mdnsrpc

import (
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"

	"github.com/armon/mdns"
)

var entries = map[string][]string{}
var entriesLock = &sync.RWMutex{}
var clients = map[string]*Client{}
var clientsLock = &sync.RWMutex{}

func lookupAll(name string) (err error) {
	addresses := []string{}
	serviceEntries := make(chan *mdns.ServiceEntry)
	done := make(chan struct{})
	go func() {
		for entry := range serviceEntries {
			addresses = append(addresses, fmt.Sprintf("%v:%v", entry.Addr.String(), entry.Port))
		}
		close(done)
	}()
	mdns.Lookup(fmt.Sprintf("_%s._tcp", name), serviceEntries)
	close(serviceEntries)
	<-done
	entriesLock.Lock()
	defer entriesLock.Unlock()
	entries[name] = addresses
	return
}

func connectAll(name string) (err error) {
	if err = lookupAll(name); err != nil {
		return
	}
	entriesLock.RLock()
	addresses := entries[name]
	entriesLock.RUnlock()
	clientsLock.Lock()
	defer clientsLock.Unlock()
	for _, addr := range addresses {
		if _, found := clients[addr]; !found {
			var client *rpc.Client
			if client, err = rpc.Dial("tcp", addr); err == nil {
				clients[addr] = &Client{
					client: client,
					addr:   addr,
				}
			}
		}
	}
	return
}

/*
A wrapper for *rpc.Client that removes the wrapped *rpc.Client from the cache if it produces errors.
*/
type Client struct {
	addr   string
	client *rpc.Client
}

func (self *Client) Call(service string, input, output interface{}) (err error) {
	if err = self.client.Call(service, input, output); err != nil {
		self.client.Close()
		clientsLock.Lock()
		defer clientsLock.Unlock()
		delete(clients, self.addr)
	}
	return
}

/*
Connect returns a *Client to the addr.
*/
func Connect(addr string) (result *Client, err error) {
	clientsLock.Lock()
	result, found := clients[addr]
	if !found {
		var rpcClient *rpc.Client
		if rpcClient, err = rpc.Dial("tcp", addr); err != nil {
			return
		}
		result = &Client{
			addr:   addr,
			client: rpcClient,
		}
		clients[addr] = result
	}
	return
}

type NoSuchService string

func (self NoSuchService) Error() string {
	return fmt.Sprintf("Failed to find any %#v", string(self))
}

type NotOneService struct {
	Name  string
	Found []string
}

func (self NotOneService) Error() string {
	return fmt.Sprintf("Failed to find exactly one %#v, found %+v", self.Name, self.Found)
}

/*
LookupOne will use mDNS to lookup exactly one service named _name._tcp and return a *Client for it.

If a lookup has already been made earlier, the cached results will be returned and then a new lookup made in the background.
*/
func LookupOne(name string) (result *Client, err error) {
	entriesLock.RLock()
	addresses := entries[name]
	entriesLock.RUnlock()
	if len(addresses) != 1 {
		if err = connectAll(name); err != nil {
			return
		}
		entriesLock.RLock()
		addresses = entries[name]
		entriesLock.RUnlock()
		if len(addresses) != 1 {
			err = NotOneService{
				Name:  name,
				Found: addresses,
			}
			return
		}
		return LookupOne(name)
	}
	go connectAll(name)
	clientsLock.RLock()
	result = clients[addresses[0]]
	clientsLock.RUnlock()
	return
}

/*
LookupAll will use mDNS to lookup all services named _name._tcp and return a slice of *Client for them.

If a lookup has already been made earlier, the cached results will be returned and then a new lookup made in the background.
*/
func LookupAll(name string) (result []*Client, err error) {
	entriesLock.RLock()
	addresses := entries[name]
	entriesLock.RUnlock()
	if len(addresses) < 1 {
		if err = connectAll(name); err != nil {
			return
		}
		entriesLock.RLock()
		addresses = entries[name]
		entriesLock.RUnlock()
		if len(addresses) < 1 {
			err = NoSuchService(name)
			return
		}
		return LookupAll(name)
	}
	clientsLock.RLock()
	defer clientsLock.RUnlock()
	for _, addr := range addresses {
		result = append(result, clients[addr])
	}
	return
}

/*
Publish will serve service using net/rpc on a randomly chosen port on 127.0.0.1, publishing it using mDNS as _name_._tcp.
*/
func Publish(name string, service interface{}) (unpublish chan struct{}, err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
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

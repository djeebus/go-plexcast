package main

import (
	"github.com/hashicorp/mdns"
	"time"
	"net"
)

type ChromecastInfo struct {
	Name		string
	Host 		string
	AddrV4		net.IP
	AddrV6		net.IP
	Port		int
}

func GetChromecasts(duration time.Duration) ([]*ChromecastInfo, error) {
	var casts []*ChromecastInfo
	ch := make(chan *mdns.ServiceEntry, 4)
	params := &mdns.QueryParam{
		Service:             "_googlecast._tcp.local.",
		Domain:              "local",
		Timeout:             duration,
		Entries:             ch,
		WantUnicastResponse: false,
	}

	go func() {
		for e := range ch {
			cast := ChromecastInfo{
				Name: e.Name,
				Host: e.Host,
				Port: e.Port,
				AddrV4: e.AddrV4,
				AddrV6: e.AddrV6,
			}
			casts = append(casts, &cast)
		}
	} ()

	err := mdns.Query(params)

	if err != nil {
		return nil, err
	}

	close(ch)
	return casts, nil
}
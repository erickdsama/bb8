// Package mdns does one-shot multicast DNS (RFC 6762 §5.1): find services on
// the home network (Home Assistant announces _home-assistant._tcp) and resolve
// "*.local" names. The Go binaries are built with CGO_ENABLED=0, so the pure-Go
// resolver cannot resolve ".local" by itself; Dial fills that gap.
//
// Queries go out from a random port, so responders answer by unicast and
// nothing has to bind 5353.
package mdns

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

var group = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// Service is one instance found by Browse.
type Service struct {
	Instance string // "Home._home-assistant._tcp.local."
	Host     string // "homeassistant.local."
	Port     int
	IPs      []net.IP
	TXT      map[string]string // key=value pairs
}

// Query builds a one-shot query packet.
func Query(name string, typ dnsmessage.Type) ([]byte, error) {
	n, err := dnsmessage.NewName(fqdn(name))
	if err != nil {
		return nil, err
	}
	m := dnsmessage.Message{Questions: []dnsmessage.Question{{Name: n, Type: typ, Class: dnsmessage.ClassINET}}}
	return m.Pack()
}

func fqdn(s string) string {
	if !strings.HasSuffix(s, ".") {
		return s + "."
	}
	return s
}

// exchange sends q and passes every answer packet to f until wait ends or f returns true.
func exchange(ctx context.Context, q []byte, wait time.Duration, f func(*dnsmessage.Message) bool) error {
	c, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return err
	}
	defer c.Close()
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = c.SetDeadline(deadline)
	if _, err := c.WriteToUDP(q, group); err != nil {
		return err
	}
	buf := make([]byte, 9000)
	for {
		n, _, err := c.ReadFromUDP(buf)
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				return nil
			}
			return err
		}
		var m dnsmessage.Message
		if m.Unpack(buf[:n]) != nil || !m.Header.Response {
			continue
		}
		if f(&m) {
			return nil
		}
	}
}

// Collect folds the records of one or more answers into services of type svc.
func Collect(msgs []*dnsmessage.Message, svc string) []Service {
	svc = strings.ToLower(fqdn(svc))
	byName := map[string]*Service{}
	hosts := map[string][]net.IP{}
	var order []string
	get := func(inst string) *Service {
		if s, ok := byName[inst]; ok {
			return s
		}
		s := &Service{Instance: inst, TXT: map[string]string{}}
		byName[inst] = s
		order = append(order, inst)
		return s
	}
	for _, m := range msgs {
		for _, rr := range append(append(m.Answers, m.Additionals...), m.Authorities...) {
			name := strings.ToLower(rr.Header.Name.String())
			switch b := rr.Body.(type) {
			case *dnsmessage.PTRResource:
				if name == svc {
					get(strings.ToLower(b.PTR.String()))
				}
			case *dnsmessage.SRVResource:
				if strings.HasSuffix(name, "."+svc) {
					s := get(name)
					s.Host, s.Port = strings.ToLower(b.Target.String()), int(b.Port)
				}
			case *dnsmessage.TXTResource:
				if strings.HasSuffix(name, "."+svc) {
					s := get(name)
					for _, t := range b.TXT {
						k, v, _ := strings.Cut(t, "=")
						s.TXT[strings.ToLower(k)] = v
					}
				}
			case *dnsmessage.AResource:
				hosts[name] = append(hosts[name], net.IP(b.A[:]))
			}
		}
	}
	var out []Service
	for _, inst := range order {
		s := byName[inst]
		s.IPs = hosts[s.Host]
		if s.Port != 0 {
			out = append(out, *s)
		}
	}
	return out
}

// Browse lists the instances of a service type, e.g. "_home-assistant._tcp.local".
func Browse(ctx context.Context, svc string, wait time.Duration) ([]Service, error) {
	q, err := Query(svc, dnsmessage.TypePTR)
	if err != nil {
		return nil, err
	}
	var msgs []*dnsmessage.Message
	err = exchange(ctx, q, wait, func(m *dnsmessage.Message) bool { msgs = append(msgs, m); return false })
	out := Collect(msgs, svc)
	// Hosts whose address did not come along: ask for them.
	for i := range out {
		if len(out[i].IPs) == 0 && out[i].Host != "" {
			if ip, err := Resolve(ctx, out[i].Host); err == nil {
				out[i].IPs = []net.IP{ip}
			}
		}
	}
	return out, err
}

// Resolve finds the IPv4 address of a ".local" name.
func Resolve(ctx context.Context, host string) (net.IP, error) {
	q, err := Query(host, dnsmessage.TypeA)
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(fqdn(host))
	var ip net.IP
	err = exchange(ctx, q, 2*time.Second, func(m *dnsmessage.Message) bool {
		for _, rr := range append(m.Answers, m.Additionals...) {
			if a, ok := rr.Body.(*dnsmessage.AResource); ok && strings.ToLower(rr.Header.Name.String()) == want {
				ip = net.IP(a.A[:])
				return true
			}
		}
		return false
	})
	if ip == nil {
		if err == nil {
			err = errors.New(host + ": no mDNS answer")
		}
		return nil, err
	}
	return ip, nil
}

var cache = struct {
	sync.Mutex
	m map[string]cached
}{m: map[string]cached{}}

type cached struct {
	ip net.IP
	at time.Time
}

// Dial is a DialContext that resolves "*.local" hosts over mDNS (cached 5 min)
// and leaves every other address to the normal dialer.
func Dial(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	host, port, err := net.SplitHostPort(addr)
	if err != nil || !strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), ".local") {
		return d.DialContext(ctx, network, addr)
	}
	key := strings.ToLower(host)
	cache.Lock()
	c, ok := cache.m[key]
	cache.Unlock()
	if !ok || time.Since(c.at) > 5*time.Minute {
		ip, err := Resolve(ctx, host)
		if err != nil {
			// The system resolver may still know it (cgo builds, /etc/hosts).
			return d.DialContext(ctx, network, addr)
		}
		c = cached{ip, time.Now()}
		cache.Lock()
		cache.m[key] = c
		cache.Unlock()
	}
	return d.DialContext(ctx, network, net.JoinHostPort(c.ip.String(), port))
}

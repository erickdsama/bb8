package mdns

import (
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func TestCollect(t *testing.T) {
	name := func(s string) dnsmessage.Name { return dnsmessage.MustNewName(s) }
	hdr := func(n string, typ dnsmessage.Type) dnsmessage.ResourceHeader {
		return dnsmessage.ResourceHeader{Name: name(n), Type: typ, Class: dnsmessage.ClassINET, TTL: 120}
	}
	m := dnsmessage.Message{Header: dnsmessage.Header{Response: true},
		Answers: []dnsmessage.Resource{{Header: hdr("_home-assistant._tcp.local.", dnsmessage.TypePTR),
			Body: &dnsmessage.PTRResource{PTR: name("Casa._home-assistant._tcp.local.")}}},
		Additionals: []dnsmessage.Resource{
			{Header: hdr("Casa._home-assistant._tcp.local.", dnsmessage.TypeSRV), Body: &dnsmessage.SRVResource{Target: name("homeassistant.local."), Port: 8123}},
			{Header: hdr("Casa._home-assistant._tcp.local.", dnsmessage.TypeTXT), Body: &dnsmessage.TXTResource{TXT: []string{"location_name=Casa", "internal_url=http://192.168.68.50:8123"}}},
			{Header: hdr("homeassistant.local.", dnsmessage.TypeA), Body: &dnsmessage.AResource{A: [4]byte{192, 168, 68, 50}}},
		}}
	// Round-trip through the wire format, as it arrives.
	b, err := m.Pack()
	if err != nil {
		t.Fatal(err)
	}
	var got dnsmessage.Message
	if err := got.Unpack(b); err != nil {
		t.Fatal(err)
	}
	s := Collect([]*dnsmessage.Message{&got}, "_home-assistant._tcp.local")
	if len(s) != 1 || s[0].Host != "homeassistant.local." || s[0].Port != 8123 || s[0].TXT["location_name"] != "Casa" ||
		len(s[0].IPs) != 1 || s[0].IPs[0].String() != "192.168.68.50" {
		t.Fatalf("%+v", s)
	}
	if q, err := Query("homeassistant.local", dnsmessage.TypeA); err != nil || len(q) < 12 {
		t.Fatal(q, err)
	}
}

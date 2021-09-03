package easemesh

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

func TestServices(t *testing.T) {

	m := New([]string{"interwebs.tests."})
	m.dnsController = &mockDNSController{}

	type svcAns struct {
		host string
		key  string
	}
	type svcTest struct {
		qname  string
		qtype  uint16
		answer []svcAns
	}
	tests := []svcTest{
		// Cluster IP Services
		{
			qname: "vets-service.spring-petclinic.svc.interwebs.test.", qtype: dns.TypeA,
			answer: []svcAns{{host: "127.0.0.1", key: "/" + coredns + "/test/interwebs/svc/spring-petclinic/vets-service"}}},
		{
			qname: "_http._tcp.vets-service.spring-petclinic.svc.interwebs.test.", qtype: dns.TypeSRV,
			answer: []svcAns{{host: "127.0.0.1", key: "/" + coredns + "/test/interwebs/svc/spring-petclinic/vets-service"}}},
		{
			qname: "vets.vets-service.spring-petclinic.svc.interwebs.test.", qtype: dns.TypeA,
			answer: []svcAns{{host: "127.0.0.1", key: "/" + coredns + "/test/interwebs/svc/spring-petclinic/vets-service/vets"}}},
	}

	for i, test := range tests {
		state := request.Request{
			Req:  &dns.Msg{Question: []dns.Question{{Name: test.qname, Qtype: test.qtype}}},
			Zone: "interwebs.test.", // must match from k.Zones[0]
		}
		svcs, e := m.Services(context.TODO(), state, false, plugin.Options{})
		if e != nil {
			t.Errorf("Test %d: got error '%v'", i, e)
			continue
		}

		if len(svcs) != len(test.answer) {
			t.Errorf("Test %d, expected %v answer, got %v", i, len(test.answer), len(svcs))
			continue
		}

		for j := range svcs {
			if test.answer[j].host != svcs[j].Host {
				t.Errorf("Test %d, expected host '%v', got '%v'", i, test.answer[j].host, svcs[j].Host)
			}

			if test.answer[j].key != svcs[j].Key {
				t.Errorf("Test %d, expected key '%v', got '%v'", j, test.answer[j].key, svcs[j].Key)
			}
		}

	}

}

type mockDNSController struct{}

var _ dnsController = &mockDNSController{}

func (m *mockDNSController) ServiceList() []*Service         { return mockServices }
func (m *mockDNSController) ServiceByName(string) []*Service { return mockServices }

var mockServices = []*Service{
	newService("vets-service", "pet", 13001),
	newService("customers-service", "pet", 13001),
	newService("svc1", "pet", 13001),
}

func newService(name, tenant string, egressPort int) *Service {
	return &Service{Name: name, Tenant: tenant, EgressPort: egressPort}
}

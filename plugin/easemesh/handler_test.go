package easemesh

import (
	"context"
	"testing"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"
	"github.com/miekg/dns"
)

func TestServeDNS(t *testing.T) {
	em := New([]string{"cluster.local."})
	em.dnsController = &mockDNSController{}
	em.Next = test.NextHandler(dns.RcodeSuccess, nil)

	ctx := context.TODO()
	for i, tc := range dnsTestCases {
		r := tc.Msg()
		w := dnstest.NewRecorder(&test.ResponseWriter{})
		_, err := em.ServeDNS(ctx, w, r)
		if err != tc.Error {
			t.Errorf("Test %d expected no error, got %v", i, err)
			return
		}
		resp := w.Msg
		if resp == nil {
			t.Fatalf("Test %d, got nil message and no error for %q", i, r.Question[0].Name)
		}

		if err := test.CNAMEOrder(resp); err != nil {
			t.Error(err)
		}

		if err := test.SortAndCheck(resp, tc); err != nil {
			t.Error(err)
		}
	}

}

var dnsTestCases = []test.Case{
	// A Service
	{
		Qname: "vets-service.spring-petclinic.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("vets-service.spring-petclinic.svc.cluster.local.	5	IN	A	127.0.0.1"),
		},
	},
	{
		Qname: "customers-service.spring-petclinic.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("customers-service.spring-petclinic.svc.cluster.local.	5	IN	A	127.0.0.1"),
		},
	},
	// A Service (wildcard)
	{
		Qname: "svc1.*.svc.cluster.local.", Qtype: dns.TypeA,
		Rcode: dns.RcodeSuccess,
		Answer: []dns.RR{
			test.A("svc1.*.svc.cluster.local.  5       IN      A       127.0.0.1"),
		},
	},
}

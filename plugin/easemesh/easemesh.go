package easemesh

import (
	"context"
	"strings"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

const loopbackIpv4 = "127.0.0.1"
const loopbackIpv6 = "::1"

// EaseMesh is client of the easegress
type EaseMesh struct {
	Next          plugin.Handler
	dnsController dnsController
	Fall          fall.F
	Zones         []string
	Upstream      *upstream.Upstream
	ttl           uint32
}

var _ plugin.ServiceBackend = &EaseMesh{}

// Services implements the ServiceBackend interface.
func (e *EaseMesh) Services(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (services []msg.Service, err error) {
	// TODO
	// We're looking again at types, which we've already done in ServeDNS, but there are some types k8s just can't answer.
	switch state.QType() {

	case dns.TypeTXT:
		// 1 label + zone, label must be "dns-version".
		t, _ := dnsutil.TrimZone(state.Name(), state.Zone)

		segs := dns.SplitDomainName(t)
		if len(segs) != 1 {
			return nil, nil
		}
		if segs[0] != "dns-version" {
			return nil, nil
		}
		svc := msg.Service{Text: "1.1.0", TTL: 28800, Key: msg.Path(state.QName(), coredns)}
		return []msg.Service{svc}, nil
	}

	services, err = e.Records(ctx, state, exact)
	if err != nil {
		return
	}

	// SRV for external services is not yet implemented, so remove those records.
	if state.QType() != dns.TypeSRV {
		return services, err
	}

	internal := []msg.Service{}
	for _, svc := range services {
		if t, _ := svc.HostType(); t != dns.TypeCNAME {
			internal = append(internal, svc)
		}
	}

	return
}

// Reverse implements the ServiceBackend interface.
func (e *EaseMesh) Reverse(ctx context.Context, state request.Request, exact bool, opt plugin.Options) (services []msg.Service, err error) {
	return e.Services(ctx, state, exact, opt)
}

// Lookup implements the ServiceBackend interface.
func (e *EaseMesh) Lookup(ctx context.Context, state request.Request, name string, typ uint16) (*dns.Msg, error) {
	return e.Upstream.Lookup(ctx, state, name, typ)
}

// IsNameError implements the ServiceBackend interface.
func (e *EaseMesh) IsNameError(err error) bool {
	return err == errKeyNotFound || err == errRequestInvalid || err == errPodRequest
}

// Records looks up records in etcd. If exact is true, it will lookup just this
// name. This is used when find matches when completing SRV lookups for instance.
func (e *EaseMesh) Records(ctx context.Context, state request.Request, exact bool) ([]msg.Service, error) {
	r, err := parseRequest(state.Name(), state.Zone)
	if err != nil {
		return nil, errRequestInvalid
	}

	if r.podOrSvc != svc {
		return nil, errPodRequest
	}

	if r.service == "" {
		return nil, errKeyNotFound
	}

	if wildcard(r.service) {
		return nil, errKeyNotFound
	}

	return e.findServices(r, state.Zone)
}

func (e *EaseMesh) findServices(r recordRequest, zone string) (services []msg.Service, err error) {
	zonePath := msg.Path(zone, coredns)
	if s := e.dnsController.ServiceByName(r.service); len(s) > 0 {
		key := strings.Join([]string{zonePath, svc, r.namespace, r.service}, "/")
		return newSidecarService(s, key), nil
	}
	return nil, errKeyNotFound
}

// Serial returns a SOA serial number to construct a SOA record.
func (e *EaseMesh) Serial(state request.Request) uint32 {
	return uint32(time.Now().Unix())
}

// MinTTL returns the minimum TTL to be used in the SOA record.
func (e *EaseMesh) MinTTL(state request.Request) uint32 {
	return e.ttl
}

func newSidecarService(s []*Service, key string) (results []msg.Service) {
	for _, i := range s {
		// First add TypeA
		ms := msg.Service{
			Host: loopbackIpv4,
			Port: i.EgressPort,
			TTL:  defaultTTL,
		}
		results = append(results, ms)
		// TypeAAA also be needed
		ms = msg.Service{
			Host: loopbackIpv6,
			Port: i.EgressPort,
			TTL:  defaultTTL,
		}
		ms.Key = key
		results = append(results, ms)
	}
	return
}

// match checks if a and b are equal taking wildcards into account.
func match(a, b string) bool {
	if wildcard(a) {
		return true
	}
	if wildcard(b) {
		return true
	}
	return strings.EqualFold(a, b)
}

// wildcard checks whether s contains a wildcard value defined as "*" or "any".
func wildcard(s string) bool {
	return s == "*" || s == "any"
}

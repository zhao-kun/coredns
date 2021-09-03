package easemesh

import (
	"errors"

	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	"github.com/miekg/dns"
)

const (
	pod = "pod"
	svc = "svc"

	coredns = "c" // used as a fake key prefix in msg.Service
)

var (
	errKeyNotFound    = errors.New("Key was not found")
	errRequestInvalid = errors.New("Request parse error")
	errPodRequest     = errors.New("Name is a pod")
)

// Service is bussiness logic unit registered in the registry of the EaseMesh
type Service struct {
	Name       string
	Tenant     string
	EgressPort int
}

type recordRequest struct {
	// The named port from the kubernetes DNS spec, this is the service part (think _https) from a well formed
	// SRV record.
	port string
	// The protocol is usually _udp or _tcp (if set), and comes from the protocol part of a well formed
	// SRV record.
	protocol string
	endpoint string

	// The service name of the easemesh
	service string
	// The namespace used in Kubernetes. (ignored in k8s)
	namespace string
	// A each name can be for a pod or a service, here we track what we've seen, either "pod" or "service".
	podOrSvc string
}

func parseRequest(name, zone string) (r recordRequest, err error) {
	// 3 Possible cases:
	// 1. _port._protocol.service.namespace.pod|svc.zone
	// 2. (endpoint): endpoint.service.namespace.pod|svc.zone
	// 3. (service): service.namespace.pod|svc.zone

	base, _ := dnsutil.TrimZone(name, zone)
	// return NODATA for apex queries
	if base == "" || base == svc || base == pod {
		return r, nil
	}
	segs := dns.SplitDomainName(base)
	r.port = "*"
	r.protocol = "*"

	// for r.name, r.namespace and r.endpoint, we need to know if they have been set or not...
	// For endpoint: if empty we should skip the endpoint check in k.get(). Hence we cannot set if to "*".
	// For name: myns.svc.cluster.local != *.myns.svc.cluster.local
	// For namespace: svc.cluster.local != *.svc.cluster.local

	// start at the right and fill out recordRequest with the bits we find, so we look for
	// pod|svc.namespace.service and then either
	// * endpoint
	// *_protocol._port

	last := len(segs) - 1
	if last < 0 {
		return r, nil
	}
	r.podOrSvc = segs[last]
	if r.podOrSvc != pod && r.podOrSvc != svc {
		// Not k8s dns, we skip it
		return r, errKeyNotFound
	}
	last--
	if last < 0 {
		return r, nil
	}

	r.namespace = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	r.service = segs[last]
	last--
	if last < 0 {
		return r, nil
	}

	// Because of ambiguity we check the labels left: 1: an endpoint. 2: port and protocol.
	// Anything else is a query that is too long to answer and can safely be delegated to return an nxdomain.
	switch last {

	case 0: // endpoint only
		r.endpoint = segs[last]
	case 1: // service and port
		r.protocol = stripUnderscore(segs[last])
		r.port = stripUnderscore(segs[last-1])

	default: // too long
		// skip none k8s dns
		return r, errKeyNotFound
	}

	return r, nil
}

// stripUnderscore removes a prefixed underscore from s.
func stripUnderscore(s string) string {
	if s[0] != '_' {
		return s
	}
	return s[1:]
}

// String returns a string representation of r, it just returns all fields concatenated with dots.
// This is mostly used in tests.
func (r recordRequest) String() string {
	s := r.port
	s += "." + r.protocol
	s += "." + r.endpoint
	s += "." + r.service
	s += "." + r.namespace
	s += "." + r.podOrSvc
	return s
}

package easemesh

import (
	"context"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/miekg/dns"
)

// ServeDNS implements the plugin.Handler interface.
func (e *EaseMesh) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	opt := plugin.Options{}
	state := request.Request{W: w, Req: r}

	zone := plugin.Zones(e.Zones).Matches(state.Name())
	if zone == "" {
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}
	qname := state.QName()
	zone = qname[len(qname)-len(zone):] // maintain case of original query
	state.Zone = zone

	var (
		records, extra []dns.RR
		err            error
	)

	switch state.QType() {
	case dns.TypeA:
		records, err = plugin.A(ctx, e, zone, state, nil, opt)
	case dns.TypeSRV:
		records, extra, err = plugin.SRV(ctx, e, zone, state, opt)
	default:
		// we just intercepts A and SRV request
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}
	if err != nil && e.IsNameError(err) {
		// ignored when query failed, passthrough to next (K8s) plugin
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}
	if err != nil {
		return plugin.BackendError(ctx, e, zone, dns.RcodeServerFailure, state, err, opt)
	}

	if len(records) == 0 {
		// ignored when query no records, passthrough to next (K8s) plugin
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *EaseMesh) Name() string { return "easemesh" }

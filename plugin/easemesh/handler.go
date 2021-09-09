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
	case dns.TypeAAAA:
		records, err = plugin.AAAA(ctx, e, zone, state, nil, plugin.Options{})
	case dns.TypeTXT:
		records, err = plugin.TXT(ctx, e, zone, state, nil, plugin.Options{})
	case dns.TypeCNAME:
		records, err = plugin.CNAME(ctx, e, zone, state, plugin.Options{})
	case dns.TypePTR:
		records, err = plugin.PTR(ctx, e, zone, state, plugin.Options{})
	case dns.TypeMX:
		records, extra, err = plugin.MX(ctx, e, zone, state, plugin.Options{})
	case dns.TypeSRV:
		records, extra, err = plugin.SRV(ctx, e, zone, state, opt)
	case dns.TypeSOA:
		if qname == zone {
			records, err = plugin.SOA(ctx, e, zone, state, plugin.Options{})
		}
	case dns.TypeAXFR, dns.TypeIXFR:
		return dns.RcodeRefused, nil
	case dns.TypeNS:
		if state.Name() == zone {
			records, extra, err = plugin.NS(ctx, e, zone, state, plugin.Options{})
			break
		}
		fallthrough
	default:
		// Do a fake A lookup, so we can distinguish between NODATA and NXDOMAIN
		fake := state.NewWithQuestion(state.QName(), dns.TypeA)
		fake.Zone = state.Zone
		_, err = plugin.A(ctx, e, zone, fake, nil, plugin.Options{})
	}
	if err != nil && e.IsNameError(err) {
		// ignored when query failed, passthrough to next (K8s) plugin
		log.Infof("EXITED Next name: %s, type: %+v, error: %+v", qname, state.QType(), err)
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}
	if err != nil {
		log.Infof("EXITED BackendError name: %s, type: %+v, error: %+v", qname, state.QType(), err)
		return plugin.BackendError(ctx, e, zone, dns.RcodeServerFailure, state, err, opt)
	}

	if len(records) == 0 {
		// ignored when query no records, passthrough to next (K8s) plugin
		log.Infof("EXITED Records is zero name: %s, type: %+v,records: %+v", qname, state.QType(), records)
		return plugin.NextOrFailure(e.Name(), e.Next, ctx, w, r)
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	m.Answer = append(m.Answer, records...)
	m.Extra = append(m.Extra, extra...)
	m.RecursionAvailable = false
	//m.RecursionDesired = false
	//m.Authoritative = true
	//m.CheckingDisabled = true
	log.Infof("EXITED Succeed name: %s, type: %+v,result: %+v", qname, state.QType(), *m)
	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

// Name implements the Handler interface.
func (e *EaseMesh) Name() string { return "easemesh" }

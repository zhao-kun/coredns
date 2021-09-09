package easemesh

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/megaease/easegress/pkg/logger"
	"github.com/megaease/easegress/pkg/object/meshcontroller/spec"
	"go.etcd.io/etcd/api/v3/mvccpb"
	etcdcv3 "go.etcd.io/etcd/client/v3"
	"gopkg.in/yaml.v2"
)

// EasegressClient is the client of the easemesh control plane
type dnsController interface {
	ServiceList() []*Service
	ServiceByName(string) []*Service
}

type easegressClient struct {
	cli        *etcdcv3.Client
	done       chan struct{}
	services   []*Service
	serviceMap map[string]*Service
}

const servicePrefix = "/mesh/service-spec/"

func newDNSController(endpoints []string, cc *tls.Config, username, password string) (dnsController, error) {
	etcdCfg := etcdcv3.Config{
		Endpoints:            endpoints,
		TLS:                  cc,
		AutoSyncInterval:     1 * time.Minute,
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Minute,
		DialKeepAliveTimeout: 1 * time.Minute,
	}
	if username != "" && password != "" {
		etcdCfg.Username = username
		etcdCfg.Password = password
	}
	client := &easegressClient{cli: nil, done: make(chan struct{})}
	go client.connectEasegress(etcdCfg)
	go client.refreshServices()
	return client, nil

}

func (e *easegressClient) connectEasegress(etcdCfg etcdcv3.Config) {
	ctx := context.Background()
	defaultDuration := time.Second * 5

	connect := func(d time.Duration) *etcdcv3.Client {
		timeoutCtx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		etcdCfg.Context = timeoutCtx
		cli, err := etcdcv3.New(etcdCfg)
		if err != nil {
			return nil
		}
		return cli
	}

	for {
		C := time.After(defaultDuration)
		select {
		case <-C:
			if e.cli != nil {
				return
			}
			e.cli = connect(defaultDuration)
		}
	}

}

func (e *easegressClient) ServiceList() []*Service {
	return e.services
}

func (e *easegressClient) ServiceByName(name string) []*Service {
	serviceMap := e.serviceMap
	if serviceMap != nil {
		if serviceMap[name] != nil {
			return []*Service{serviceMap[name]}
		}
	}
	return nil
}

func (e *easegressClient) refreshServices() {
	ctx := context.Background()
	defaultDuration := time.Second * 5
	defer func() {
		e.done = nil
	}()

	duration := defaultDuration
	for {
		C := time.After(duration)
		select {
		case <-C:
			start := time.Now()
			services, serviceMap := e.fetchServices(ctx)
			e.services = services
			e.serviceMap = serviceMap
			now := time.Now()
			elapse := now.Sub(start)
			if elapse > defaultDuration {
				duration = time.Microsecond * 1
			} else {
				duration = defaultDuration - elapse
			}
		case <-e.done:
			return
		}
	}
}

func (e *easegressClient) fetchServices(ctx context.Context) ([]*Service, map[string]*Service) {

	// etcd client isn't ready
	if e.cli == nil {
		return nil, nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	resp, err := getPrefix(timeoutCtx, e.cli, servicePrefix)
	if err != nil {
		log.Errorf("fetch %s error %s", servicePrefix, err)
		return nil, nil
	}
	services := []*Service{}
	serviceMap := map[string]*Service{}
	for _, v := range resp {
		serviceSpec := &spec.Service{}
		err := yaml.Unmarshal([]byte(v), serviceSpec)
		if err != nil {
			logger.Errorf("BUG: unmarshal %s to yaml failed: %v", v, err)
			continue
		}
		service := &Service{
			Name:       serviceSpec.Name,
			Tenant:     serviceSpec.RegisterTenant,
			EgressPort: serviceSpec.Sidecar.EgressPort,
		}
		services = append(services, service)
		serviceMap[serviceSpec.Name] = service

	}
	return services, serviceMap
}

func getPrefix(ctx context.Context, cli *etcdcv3.Client, prefix string) (map[string]string, error) {
	kvs := make(map[string]string)
	rawKVs, err := getRawPrefix(ctx, cli, prefix)
	if err != nil {
		return kvs, err
	}

	for _, kv := range rawKVs {
		kvs[string(kv.Key)] = string(kv.Value)
	}

	return kvs, nil
}

func getRawPrefix(ctx context.Context, cli *etcdcv3.Client, prefix string) (map[string]*mvccpb.KeyValue, error) {
	kvs := make(map[string]*mvccpb.KeyValue)

	resp, err := cli.Get(ctx, prefix, etcdcv3.WithPrefix())
	if err != nil {
		return kvs, err
	}

	for _, kv := range resp.Kvs {
		kvs[string(kv.Key)] = kv
	}

	return kvs, nil
}

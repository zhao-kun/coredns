package easemesh

import (
	"crypto/tls"
	"errors"
	"strconv"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/dnsutil"
	mwtls "github.com/coredns/coredns/plugin/pkg/tls"
	"github.com/coredns/coredns/plugin/pkg/upstream"
)

const (
	defaultEndpoint = "http://localhost:2379"
	defaultTTL      = 5
)

func init() { plugin.Register("easemesh", setup) }

// MeshConfig contains the configuration for creating new EaseMesh plugin
type MeshConfig struct {
	TLSConfig   *tls.Config
	Endpoints   []string
	Username    string
	Password    string
	Zones       []string
	FallThrough []string
	TTL         uint32
}

func setup(c *caddy.Controller) error {
	config, err := easemeshParse(c)
	if err != nil {
		return plugin.Error("easemesh", err)
	}

	e, err := NewFromConfig(config)
	if err != nil {
		return plugin.Error("easemesh", err)
	}

	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		e.Next = next
		return e
	})

	return nil
}

func easemeshParse(c *caddy.Controller) (config MeshConfig, err error) {
	i := 0
	for c.Next() {
		if i > 0 {
			return config, plugin.ErrOnce
		}
		i++
		config, err = parseMeshConfig(c)
		if err != nil {
			return
		}
	}
	return config, nil
}

func parseMeshConfig(c *caddy.Controller) (config MeshConfig, err error) {
	zones := plugin.OriginsFromArgsOrServerBlock(c.RemainingArgs(), c.ServerBlockKeys)
	primaryZoneIndex := -1
	for i, z := range zones {
		if dnsutil.IsReverse(z) > 0 {
			continue
		}
		primaryZoneIndex = i
		break
	}

	if primaryZoneIndex == -1 {
		return config, errors.New("non-reverse zone name must be used")
	}
	config.Zones = zones

	for c.NextBlock() {
		switch c.Val() {
		case "endpoint":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return config, c.ArgErr()
			}
			config.Endpoints = args
		case "tls": // cert key cacertfile
			args := c.RemainingArgs()
			config.TLSConfig, err = mwtls.NewTLSConfigFromArgs(args...)
			if err != nil {
				return config, err
			}
		case "fallthrough":
			config.FallThrough = c.RemainingArgs()
		case "ttl":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return config, c.ArgErr()
			}
			t, err := strconv.Atoi(args[0])
			if err != nil {
				return config, err
			}
			if t < 0 || t > 3600 {
				return config, c.Errf("ttl must be in range [0, 3600]: %d", t)
			}
			config.TTL = uint32(t)
		case "credentials":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return config, c.ArgErr()
			}
			if len(args) != 2 {
				return config, c.Errf("credentials requires 2 arguments, username and password")
			}
			config.Username, config.Password = args[0], args[1]
		default:
			if c.Val() != "}" {
				return config, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	return config, nil
}

// New returns a initialized easemesh. It's
func New(zones []string) *EaseMesh {
	m := new(EaseMesh)
	m.Zones = zones
	m.ttl = defaultTTL
	return m
}

// NewFromConfig creates the EaseMesh plugin from configuration
func NewFromConfig(config MeshConfig) (*EaseMesh, error) {

	easemesh := EaseMesh{}
	controller, err := newDNSController(config.Endpoints, config.TLSConfig,
		config.Username, config.Password)
	if err != nil {
		return &EaseMesh{}, err
	}

	if len(config.FallThrough) > 0 {
		easemesh.Fall.SetZonesFromArgs(config.FallThrough)
	}

	if config.TTL > 0 {
		easemesh.ttl = config.TTL
	}

	easemesh.Upstream = upstream.New()
	easemesh.dnsController = controller
	return &easemesh, nil
}

package easemesh

import (
	"strings"
	"testing"

	"github.com/coredns/caddy"
)

func TestSetupEtcd(t *testing.T) {
	tests := []struct {
		input              string
		shouldErr          bool
		expectedZones      []string
		expectedEndpoint   []string
		expectedErrContent string // substring from the expected error. Empty for positive cases.
		username           string
		password           string
	}{
		{
			`easemesh coredns.local test.local {
	endpoint http://localhost:2379 http://localhost:3379 http://localhost:4379
}`,
			false, []string{"coredns.local", "test.local"}, []string{"http://localhost:2379", "http://localhost:3379", "http://localhost:4379"}, "", "", "",
		},
		{
			`easemesh skydns.local {
			endpoint http://localhost:2379
			credentials username password
		}
			`, false, []string{"skydns.local"}, []string{"http://localhost:2379"}, "", "username", "password",
		},
		// with credentials, missing zones
		{
			`easemesh {
			endpoint http://localhost:2379
		}
			`, true, []string{}, []string{"http://localhost:2379"}, "non-reverse zone name must be used", "", "",
		},
		// with credentials, missing username and  password
		{
			`easemesh skydns.local {
			endpoint http://localhost:2379
			credentials
		}
			`, true, []string{"skydns.local"}, []string{"http://localhost:2379"}, "Wrong argument count", "", "",
		},
	}

	for i, test := range tests {
		c := caddy.NewTestController("dns", test.input)
		easemesh, err := easemeshParse(c)

		if test.shouldErr && err == nil {
			t.Errorf("Test %d: Expected error but found %s for input %s", i, err, test.input)
		}

		if err != nil {
			if !test.shouldErr {
				t.Errorf("Test %d: Expected no error but found one for input %s. Error was: %v", i, test.input, err)
				continue
			}

			if !strings.Contains(err.Error(), test.expectedErrContent) {
				t.Errorf("Test %d: Expected error to contain: %v, found error: %v, input: %s", i, test.expectedErrContent, err.Error(), test.input)
				continue
			}
		}

		if !test.shouldErr {
			if len(easemesh.Endpoints) != len(test.expectedEndpoint) {
				t.Errorf("Etcd not correctly set for input %s. Expected: '%+v', actual: '%+v'", test.input, test.expectedEndpoint, easemesh.Endpoints)
			}
			for i, endpoint := range easemesh.Endpoints {
				if endpoint != test.expectedEndpoint[i] {
					t.Errorf("Etcd not correctly set for input %s. Expected: '%+v', actual: '%+v'", test.input, test.expectedEndpoint, easemesh.Endpoints)
				}
			}
		}

		if !test.shouldErr {
			if test.username != "" {
				if easemesh.Username != test.username {
					t.Errorf("Etcd username not correctly set for input %s. Expected: '%+v', actual: '%+v'", test.input, test.username, easemesh.Username)
				}
			}
			if test.password != "" {
				if easemesh.Password != test.password {
					t.Errorf("Etcd password not correctly set for input %s. Expected: '%+v', actual: '%+v'", test.input, test.password, easemesh.Password)
				}
			}
		}
	}
}

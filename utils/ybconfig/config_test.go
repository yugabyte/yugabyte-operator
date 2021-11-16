package ybconfig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFromJSON(t *testing.T) {

	oneHostCfg := `{"version":160,"serverBlacklist":{"hosts":[{"host":"yb-tserver-5.yb-tservers.yb-operator.svc.cluster.local","port":9100}],"initialReplicaLoad":0},"clusterUuid":"e3dcb2a9-c1e7-4c58-937d-73f9583b5132"}`

	twoHostsCfg := `{"version":145,"serverBlacklist":{"hosts":[{"host":"yb-tserver-5.yb-tservers","port":9100},{"host":"yb-tserver-4.yb-tservers","port":9200}]},"clusterUuid":"e3dcb2a9-c1e7-4c58-937d-73f9583b5132"}`

	blankBl := `{"version":123,"clusterUuid":"e3dcb2a9-c1e7-4c58-937d-73f9583b5132"}`

	cases := []struct {
		name string
		cfg  string
		len  int
		err  bool
	}{
		{"one host", oneHostCfg, 1, false},
		{"two hosts", twoHostsCfg, 2, false},
		{"blank serverBlacklist", blankBl, 0, false},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			cfgObj, err := NewFromJSON([]byte(cas.cfg))
			if cas.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Len(t, cfgObj.ServerBlacklist.Hosts, cas.len)

		})
	}
}

func TestGetBlacklist(t *testing.T) {

	oneHostCfg := UniverseConfig{
		ServerBlacklist: ServerBlacklist{
			Hosts: []Host{
				{Host: "yb-tserver-5.yb-tservers", Port: 1234},
			},
		},
	}

	twoHostsCfg := UniverseConfig{
		ServerBlacklist: ServerBlacklist{
			Hosts: []Host{
				{Host: "yb-tserver-5.yb-tservers", Port: 1234},
				{Host: "yb-tserver-4.yb-chart", Port: 111},
			},
		},
	}

	blankBl := UniverseConfig{}

	cases := []struct {
		name     string
		obj      UniverseConfig
		expected []string
	}{
		{"one host", oneHostCfg, []string{"yb-tserver-5.yb-tservers:1234"}},
		{"two hosts", twoHostsCfg, []string{"yb-tserver-5.yb-tservers:1234", "yb-tserver-4.yb-chart:111"}},
		{"blank serverBlacklist", blankBl, nil},
	}

	for _, cas := range cases {
		t.Run(cas.name, func(t *testing.T) {
			got := cas.obj.GetBlacklist()
			assert.Equal(t, cas.expected, got)
		})
	}
}

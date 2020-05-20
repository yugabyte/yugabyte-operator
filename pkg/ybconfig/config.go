package ybconfig

import (
	"encoding/json"
	"fmt"
)

// UniverseConfig defines YugabyteDB universe configuraion.
type UniverseConfig struct {
	ServerBlacklist ServerBlacklist `json:"serverBlacklist,omitempty"`
}

// ServerBlacklist defines details about blacklisted server.
type ServerBlacklist struct {
	// Hosts has a list of blacklisted servers
	Hosts []Host `json:",omitempty"`
}

// Host defines details about a server.
type Host struct {
	// Host is hostname of the server
	Host string
	// Port is the port on which server is listening
	Port int
}

// GetBlacklist returns a list of blacklisted server in hostname:port
// format.
func (c UniverseConfig) GetBlacklist() []string {
	var bl []string
	for _, host := range c.ServerBlacklist.Hosts {
		bl = append(bl, fmt.Sprintf("%s:%d", host.Host, host.Port))
	}
	return bl
}

// NewFromJSON parses JSON-encoded data and returns
// UniverseConfig. If the parsing fails, it returns error.
func NewFromJSON(data []byte) (UniverseConfig, error) {
	cfg := UniverseConfig{}
	err := json.Unmarshal(data, &cfg)
	return cfg, err
}

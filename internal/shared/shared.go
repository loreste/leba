package shared

// PeerConfig represents configuration for a peer node
type PeerConfig struct {
	Address string `yaml:"address"`
	NodeID  string `yaml:"node_id"`
}

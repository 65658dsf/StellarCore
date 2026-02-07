package v1

type VirtualNetConfig struct {
	Address string `json:"address,omitempty"`
}

func (c *VirtualNetConfig) Complete() {}

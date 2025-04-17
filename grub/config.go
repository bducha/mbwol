package grub

import (
	"errors"
	"sync"
	"time"
)

const (
	ERR_HOST_NOT_FOUND = "host_not_found"
	ERR_CONFIG_NOT_FOUND = "config_not_found"
)

type Host struct {
	ID string
	IP string
	Configs map[string]string
	CurrentConfig *string
	LastSetAt *time.Time
	Timeout uint8
	ResetAfterGet bool
}

type HostConfigs struct {
	mu sync.Mutex
	hosts map[string]Host
}

var hc *HostConfigs

func InitHostConfigs() {
	hc = &HostConfigs{
		hosts: map[string]Host{
			"main":{
				ID: "main",
				IP: "10.0.2.1",
				Configs: map[string]string{
					"arch": "set default=0\nset timeout=1\n",
					"windows": "set timeout=1\n",
				},
				Timeout: 60,
				ResetAfterGet: true,
			},
		},
	}
}

// Returns the currentConfig for the host
// Returns an empty string if the host is not found
func GetConfigByIp(clientIp string) string {

	hc.mu.Lock()
	defer hc.mu.Unlock()
	
	var hostId string
	hostFound := false

	for id, c := range(hc.hosts) {
		if c.IP == clientIp {
			hostFound = true
			hostId = id
			break
		}
	}

	if  !hostFound {
		return ""
	}

	host := hc.hosts[hostId]

	if host.CurrentConfig == nil {
		return ""
	}

	configContent := host.Configs[*host.CurrentConfig]

	t := time.Now()
	defer func () {
		host.LastSetAt = &t
		if host.ResetAfterGet {
			host.CurrentConfig = nil
		}
		hc.hosts[host.ID] = host

	}()

	if host.LastSetAt == nil || host.Timeout == 0 {
		return configContent
	}

	if time.Since(*host.LastSetAt).Seconds() <= float64(host.Timeout) {
		return configContent
	}

	return ""
}

func SetCurrentConfig(id string, configName string) error {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	host, ok := hc.hosts[id]
	if !ok {
		return errors.New(ERR_HOST_NOT_FOUND)
	}

	_, ok = host.Configs[configName]
	if !ok {
		return errors.New(ERR_CONFIG_NOT_FOUND)
	}

	host.CurrentConfig = &configName
	t := time.Now()
	host.LastSetAt = &t

	hc.hosts[id] = host

	return nil
}


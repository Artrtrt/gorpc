package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type Config struct {
	Ip         string `json:"ip"`
	TcpPort    string `json:"tcp_port"`
	HttpPort   string `json:"http_port"`
	HubIp      string `json:"hub_ip"`
	HubUdpPort string `json:"hub_udp_port"`
	IsLocal    bool   `json:"is_local"`
}

func NewConfig(path string) (conf *Config, err error) {
	_, err = os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultConf(), nil
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return
	}

	defer file.Close()

	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		return
	}

	err = json.Unmarshal(byteValue, &conf)
	if err != nil {
		return
	}

	return
}

func defaultConf() *Config {
	return &Config{
		Ip:         "localhost",
		TcpPort:    "8083",
		HttpPort:   "8084",
		HubIp:      "localhost",
		HubUdpPort: "2000",
		IsLocal:    true,
	}
}

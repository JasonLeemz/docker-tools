package tools

import (
	"encoding/json"
	"fmt"
)

type Nets []Net

type Net struct {
	Containers Containers `json:"Containers"`
}

type Containers map[string]Container

type Container struct {
	Name        string `json:"Name"`
	IPv4Address string `json:"IPv4Address"`
	IPv6Address string `json:"IPv6Address"`
}

func GetNetworkList() ([]string, error) {
	cmd := "docker network ls"
	o, _ := Command(cmd)
	fmt.Println(o)
	return nil, nil
}

func GetIPList() ([]string, error) {
	cmd := "docker network inspect bridge"
	o, _ := Command(cmd)

	n := Nets{}
	json.Unmarshal(o, &n)
	net := n[0]

	for _, container := range net.Containers {
		fmt.Println(container.Name, container.IPv4Address, container.IPv6Address)
	}
	return nil, nil
}

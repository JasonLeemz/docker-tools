package tools

import (
	"encoding/json"
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/JasonLeemz/docker-tools/core/log"
	"os"
	"strings"
	"text/template"
)

var config Config

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

type Config struct {
	Networks struct {
		Name string `toml:"name"`
	} `toml:"networks"`
	Nginx struct {
		Conf string `toml:"conf"`
		Tpl  string `toml:"tpl"`
	} `toml:"nginx"`
	Port map[string]string `toml:"port"`
}

func NginxReload() error {
	logger := log.InitLogger()
	cmd := "docker exec openresty nginx -s reload"
	logger.Info(cmd)

	o, err := Command(cmd)
	logger.Info("NginxReload:", string(o), err)
	return err
}

func UpdateProxy() error {
	logger := log.InitLogger()

	ips, err := GetIPList()
	logger.Debug("GetIPList:", ips)

	t, err := template.ParseFiles(config.Nginx.Tpl)
	if err != nil {
		logger.Warnf("ParseTemplate err:%v", err)
		return err
	}

	confd := config.Nginx.Conf
	for name, ip := range ips {
		//输出文件
		outFile := confd + "/" + name + ".conf"
		err = os.RemoveAll(outFile)
		if err != nil {
			logger.Warnf("Remove file:%v", err)
		}

		file, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			panic(err)
		}

		// 从配置文件中获取版本号
		port := ""
		ok := false
		if port, ok = config.Port[name]; !ok {
			logger.Warnf("%s not found", name)
			continue
		}
		err = t.Execute(file, map[string]interface{}{
			"name": name,
			"ip":   ip,
			"port": port,
		})
		if err != nil {
			logger.Warnf("ExecuteTemplate err:%v", err)
			return err
		}
	}

	return nil
}
func GetIPList() (map[string]string, error) {
	logger := log.InitLogger()

	cmd := "docker network inspect " + config.Networks.Name
	logger.Debug("cmd:%s", cmd)

	o, _ := Command(cmd)

	n := Nets{}
	json.Unmarshal(o, &n)
	if len(n) == 0 {
		return nil, errors.New("no ip")
	}

	net := n[0]
	ips := make(map[string]string)
	for _, container := range net.Containers {
		addr := strings.Split(container.IPv4Address, "/")
		ip := addr[0]
		ips[container.Name] = ip
	}

	logger.Info(ips)
	return ips, nil
}

func init() {
	logger := log.InitLogger()

	// config/app.toml
	path := "config/app.toml"
	if _, err := os.Stat(path); err != nil {
		logger.Warnf("ParseToml err:%v", err)
		panic(err)
	}

	// 解析配置文件
	_, err := toml.DecodeFile(path, &config)

	if err != nil {
		logger.Warnf("DecodeFile err:%v", err)
		panic(err)
	}

	logger.Debug(config)
}

package tools

import (
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/BurntSushi/toml"
	"github.com/JasonLeemz/docker-tools/core/log"
	_ "github.com/mattn/go-sqlite3"
	"os"
	"strings"
	"text/template"
	"time"
)

var config Config
var db *sql.DB

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
		HttpConf   string `toml:"http_conf"`
		HttpTpl    string `toml:"http_tpl"`
		StreamConf string `toml:"stream_conf"`
		StreamTpl  string `toml:"stream_tpl"`
	} `toml:"nginx"`
	PortHttp   map[string]string   `toml:"port_http"`
	PortStream map[string][]string `toml:"port_stream"`
}

func NginxReload() error {
	logger := log.InitLogger()
	cmd := "docker exec openresty nginx -s reload"
	logger.Info(cmd)

	o, err := Command(cmd)
	logger.Info("NginxReload:", string(o), err)
	return err
}

func UpdateProxy() (bool, error) {
	logger := log.InitLogger()
	reload := false

	ips, err := GetIPList()
	logger.Debug("GetIPList:", ips)

	// 获取http.conf模板
	httpTpl, err := template.ParseFiles(config.Nginx.HttpTpl)
	if err != nil {
		logger.Warnf("Parse HttpTpl err:%v", err)
		return reload, err
	}

	// 获取tcp.conf模板
	streamTpl, err := template.ParseFiles(config.Nginx.StreamTpl)
	if err != nil {
		logger.Warnf("Parse TcpTpl err:%v", err)
		return reload, err
	}

	// http虚拟主机配置文路径
	confd := config.Nginx.HttpConf
	// stream端口转发配置文件路径
	tcpd := config.Nginx.StreamConf

	// 比对ip，生成配置文件
	reload, err = generateConfig(ips, httpTpl, streamTpl, confd, tcpd)

	defer db.Close()
	return reload, nil
}
func GetIPList() (map[string]string, error) {
	logger := log.InitLogger()

	cmd := "docker network inspect " + config.Networks.Name
	logger.Debugf("cmd:%s", cmd)

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

func getServerIPCache() (map[string]string, error) {
	logger := log.InitLogger()

	//查询数据
	rows, err := db.Query("SELECT `name`,`IP` FROM serverip")
	if err != nil {
		return nil, err
	}
	sips := make(map[string]string, 0)
	for rows.Next() {
		var name string
		var ip string
		err = rows.Scan(&name, &ip)
		if err != nil {
			logger.Error(err)
			return nil, err
		}
		sips[name] = ip
	}

	return sips, nil
}

func init() {
	logger := log.InitLogger()

	// config/app.toml
	path := "config/app.toml"
	//path := "/Users/limingze/GolandProjects/docker-tool/config/app.toml"
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

	// 初始化sqlite
	//db, err = sql.Open("sqlite3", "/Users/limingze/GolandProjects/docker-tool/cache/cache.db")
	db, err = sql.Open("sqlite3", "cache/cache.db")
	//db, err = sql.Open("sqlite3", "cache/cache.db")
	if err != nil {
		logger.Errorf("初始化sqlite err:%v", err)
	}

	sql_table := `CREATE TABLE IF NOT EXISTS serverip(
id INTEGER PRIMARY KEY AUTOINCREMENT,name VARCHAR(64) NULL,IP VARCHAR(64) NULL,utime datetime NOT NULL DEFAULT CURRENT_TIMESTAMP);`

	res, err := db.Exec(sql_table)

	logger.Debugf("sql_table:%s,CREATE TABLE RESULT:%v,ERR:%v", sql_table, res, err)
}

func generateConfig(ips map[string]string, httpTpl, streamTpl *template.Template, confd, tcpd string) (bool, error) {
	logger := log.InitLogger()
	var err error
	reload := false

	// 获取当前macvlan的ip地址
	mip, merr := getServerIPCache()

	for name, ip := range ips {
		// 判断ip是否变化
		sip, ok := mip[name]
		if merr == nil && ok != false && sip == ip {
			// 没有变化
			continue
		}

		logger.Debugf("range ips,name:%s, ip:%s", name, ip)

		// 输出文件
		// http 配置
		outFile := confd + "/" + name + ".conf"
		err = os.RemoveAll(outFile)
		if err != nil {
			logger.Warnf("Remove file:%v", err)
		}
		// 生成http配置文件
		httpFile, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			panic(err)
		}

		// 从配置文件中获取端口号
		port := ""
		ok = false
		if port, ok = config.PortHttp[name]; !ok {
			logger.Warnf("http %s not found", name)
		} else {
			// 注入变量，生成完整配置
			err = httpTpl.Execute(httpFile, map[string]interface{}{
				"name": name,
				"ip":   ip,
				"port": port,
			})
			if err != nil {
				logger.Warnf("Execute Http Template err:%v", err)
				return reload, err
			}
		}

		// =========Http End=========

		// tcp 配置
		outFile = tcpd + "/" + name + ".conf"
		err = os.RemoveAll(outFile)
		if err != nil {
			logger.Warnf("Remove file:%v", err)
		}
		// 生成tcp配置文件
		tcpFile, err := os.OpenFile(outFile, os.O_CREATE|os.O_WRONLY, 0755)
		if err != nil {
			panic(err)
		}

		// 从配置文件中获取端口号
		ports := make([]string, 0)
		ok = false
		if ports, ok = config.PortStream[name]; !ok {
			logger.Warnf("tcp %s not found", name)
			// 以Http配置为主，这里报错后不阻断运行
		} else {
			// 注入变量，生成完整配置
			err = streamTpl.Execute(tcpFile, map[string]interface{}{
				"ports": ports,
				"ip":    ip,
				"name":  name,
			})
			if err != nil {
				logger.Warnf("Execute Tcp Template err:%v", err)
				// 以Http配置为主，这里报错后不阻断运行
				//return reload, err
			}
		}

		// =========Tcp End=========

		// 更新cache
		reload = true // 只要有ip发生变化，就需要reload
		stmt, err := db.Prepare("delete from serverip where `name` =?")
		logger.Infof("db delete err:%v", err)
		res, err := stmt.Exec(name)
		logger.Infof("db delete res:%v, err:%v", res, err)

		stmt, err = db.Prepare("INSERT INTO serverip (`name`,`IP`,`utime`) VALUES(?,?,?)")
		if err != nil {
			logger.Errorf("db.Prepare err:%v", err)
		}

		shanghaiZone, _ := time.LoadLocation("Asia/Shanghai")
		formatTimeStr := time.Now().Format("2006-01-02 15:04:05")
		formatTime, _ := time.ParseInLocation("2006-01-02 15:04:05", formatTimeStr, shanghaiZone)

		res, err = stmt.Exec(name, ip, formatTime)
		logger.Infof("db Exec res:%v, err:%v", res, err)
	}

	return reload, err
}

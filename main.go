package main

import (
	"github.com/JasonLeemz/docker-tools/core/log"
	"github.com/JasonLeemz/docker-tools/tools"
)

func main() {
	//实例化日志类
	logger := log.InitLogger()

	reload, err := tools.UpdateProxy()
	if err != nil {
		panic(err)
	}
	logger.Infof("nginx reload:%t,err:%v", reload, err)

	// nginx -s reload
	if reload {
		err = tools.NginxReload()
		if err != nil {
			panic(err)
		}
	}

	defer logger.Sync() // 将 buffer 中的日志写到文件中

}

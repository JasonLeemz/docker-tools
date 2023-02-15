package main

import (
	"github.com/JasonLeemz/docker-tools/core/log"
	"github.com/JasonLeemz/docker-tools/tools"
)

func main() {
	//实例化日志类
	logger := log.InitLogger()

	err := tools.UpdateProxy()
	if err != nil {
		panic(err)
	}

	err = tools.NginxReload()
	if err != nil {
		panic(err)
	}

	defer logger.Sync() // 将 buffer 中的日志写到文件中

}

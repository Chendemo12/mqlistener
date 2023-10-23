package main

import (
	"flag"
	"gitee.com/chendemo12/mqlistener/src/config"
	"gitee.com/chendemo12/mqlistener/src/monitor"
	"github.com/Chendemo12/functools/environ"
	"os"
	"os/signal"
)

var filepath = ""

func init() {
	flag.StringVar(&filepath, "conf", "", "specifying a config file.")
	flag.Parse()
}

func main() { Server() }

func Server() {
	// 配置文件路径
	cFilepath := environ.GetString("CONFIG_FILEPATH", "./config.yaml")
	if filepath != "" {
		cFilepath = filepath
	}

	conf, err := config.LoadConfig(cFilepath)
	if err != nil {
		panic(err)
	}

	// 覆盖默认配置
	conf.Broker.Host = environ.GetString("BROKER_LISTEN_HOST", conf.Broker.Host)
	conf.Broker.Port = environ.GetString("BROKER_LISTEN_PORT", conf.Broker.Port)
	conf.Broker.Token = environ.GetString("BROKER_TOKEN", conf.Broker.Token)
	conf.Name = environ.GetString(conf.Name, "DEFAULT")
	conf.Debug = environ.GetBool("DEBUG", conf.Debug)

	// 启动服务
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	m := monitor.New(conf)

	m.Start()

	<-quit // 阻塞进程，直到接收到停止信号,准备关闭程序
	m.Stop()
}

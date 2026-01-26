package main

import (
	"fmt"
	"kama_chat_server/internal/config"
	"kama_chat_server/internal/dao"
	"kama_chat_server/internal/https_server"
	"kama_chat_server/internal/service/chat"
	"kama_chat_server/internal/service/gorm"
	"kama_chat_server/internal/service/kafka"
	myredis "kama_chat_server/internal/service/redis"
	"kama_chat_server/pkg/zlog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	conf := config.GetConfig()
	host := conf.MainConfig.Host
	port := conf.MainConfig.Port
	kafkaConfig := conf.KafkaConfig

	userDAO := dao.NewUserDAO(dao.GormDB)
	groupDAO := dao.NewGroupDAO(dao.GormDB)
	messageDAO := dao.NewMessageDAO(dao.GormDB)
	sessionDAO := dao.NewSessionDAO(dao.GormDB)

	gorm.InitSessionService(sessionDAO, userDAO, groupDAO)
	gorm.InitUserInfoService(userDAO)
	gorm.InitGroupInfoService(groupDAO, userDAO)
	gorm.InitMessageService(messageDAO)
	if kafkaConfig.MessageMode == "kafka" {
		kafka.KafkaService.KafkaInit()
	}

	if kafkaConfig.MessageMode == "channel" {
		go chat.ChatServer.Start()
	} else {
		go chat.KafkaChatServer.Start()
	}

	go func() {
		// Win10本地部署
		// if err := https_server.GE.RunTLS(fmt.Sprintf("%s:%d", host, port), "pkg/ssl/127.0.0.1+2.pem", "pkg/ssl/127.0.0.1+2-key.pem"); err != nil {
		// 	zlog.Fatal("server running fault")
		// 	return
		// }
		// Ubuntu22.04云服务器部署
		//fmt.Println("test host and port is  %s:%d", host, port)
		//if err := https_server.GE.RunTLS(fmt.Sprintf("%s:%d", host, port), "/etc/ssl/certs/server.crt", "/etc/ssl/private/server.key"); err != nil {
		//	zlog.Fatal("server running fault")
		//	return
		//}
		fmt.Printf("test host and port is %s:%d\n", host, port)
		if err := https_server.GE.Run(fmt.Sprintf("%s:%d", host, port)); err != nil {
			zlog.Fatal("server running fault")
			return
		}
	}()

	// 设置信号监听
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	<-quit

	if kafkaConfig.MessageMode == "kafka" {
		kafka.KafkaService.KafkaClose()
	}

	chat.ChatServer.Close()

	zlog.Info("关闭服务器...")

	// 删除所有Redis键
	if err := myredis.DeleteAllRedisKeys(); err != nil {
		zlog.Error(err.Error())
	} else {
		zlog.Info("所有Redis键已删除")
	}

	zlog.Info("服务器已关闭")

}

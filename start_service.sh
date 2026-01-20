#!/bin/bash

service mysql start
service apache2 start
service redis-server start

cd /root/gochat/KamaChat/cmd/kama_chat_server || { echo "切换目录失败"; exit 1; }

if [ -f "main" ]; then
    chmod +x main
    echo "项目启动中..."
    ./main
else
    echo "错误：未找到编译后的 main 文件"
fi
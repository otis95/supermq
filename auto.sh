#!/bin/bash

echo "start svn update"
svn update 
echo "start svn update ok"

sleep 1

echo "start compiler"
go build -ldflags "-X 'main.TimeStamp=$(date -R)' -X 'main.Version=0.2.0' -X 'main.SVNVersion=$(svn info | grep "vision" | awk -F':' '{print $2}')'"
echo "start compiler ok"

sleep 1

echo "kill & cp & run"

killall -9 surgemq

tar czvf surgemq0.2.1-`date +%Y%m%d%H%M%S`.tar.gz  surgemq  appserver_dev.conf appserver_prod.conf appserver_test.conf certs/michael.crt certs/michael.key

#cp -f surgemq /home/lowan/surgemq/
#cp -f appserver_dev.conf  /home/lowan/surgemq/
#cp -rf certs  /home/lowan/surgemq/

#cd /home/lowan/surgemq/

#nohup /home/lowan/surgemq/surgemq -c  appserver_dev.conf >server.log 2>&1 &

echo "kill & cp & run ok"
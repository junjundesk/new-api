#!/usr/bin/env bash
nohup bash /tmp/newapi-deploy.sh > /root/newapi-deploy.log 2>&1 &
echo "deploy pid=$!"

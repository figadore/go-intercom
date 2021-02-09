#!/bin/bash
host=$1

if [[ $host == "201" || $host == "200" ]]; then
  scp compiled/gointercom_arm7 $host:~/
fi
if [[ $host == "202" || $host == "203" ]]; then
  scp compiled/gointercom_arm6 $host:~/
fi

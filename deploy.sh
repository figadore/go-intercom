#!/bin/bash

for host in "$@"
do
  if [[ $host == "201" || $host == "200" ]]; then
    scp compiled/gointercom_arm7 $host:~/
  fi
  if [[ $host == "202" || $host == "203" ]]; then
    scp compiled/gointercom_arm6 $host:~/
  fi
done

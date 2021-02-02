#!/bin/bash
host=$1

ssh -t $host "~/gointercom_arm\$(expr substr \$(uname -m) 5 1)"

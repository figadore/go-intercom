#!/bin/bash
clientserver=$1
host=$2

ssh -t $host "~/gointercom-${clientserver}_arm\$(expr substr \$(uname -m) 5 1)"

#!/bin/bash
host=$1

ssh $host '~/gointercom-arm$(expr substr $(uname -m) 5 1)'

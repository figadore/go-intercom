# Go Intercom (for Raspberry Pi)

## Build
~gox~
./build.sh
makefile?

## Deploy
scp

## Run
copy <repo>.env.dist to <remote>:<dir>.env

set variables in .env in the directory where binaries are deployed (see github.com/joho/godotenv)

run `<dir>/gointercom-arm$(expr substr $(uname -m) 5 1)`

# Go Intercom (for Raspberry Pi)

## Build
`./build.sh`
makefile?

## Deploy
run `./deploy.sh <host>` to copy the binaries to the device

## Run
copy <repo>/.env.dist to <remote>:<dir>.env (currently only works with run.sh if at root of home)

set variables in .env in the directory where binaries are deployed (see github.com/joho/godotenv)

run `./run.sh <host>` to run the binary through ssh

# Go Intercom (for Raspberry Pi)

## Build
`./build.sh`

## Deploy
run `./deploy.sh <host>` to copy the binaries to the device

## Run
### Setup
copy <repo>/.env.dist to <remote>:<dir>.env (currently only works with run.sh if at root of home)

set variables in .env in the directory where binaries are deployed (see github.com/joho/godotenv)

### Run

run `./run.sh <'client'|'server'> <host>` to run the binary through ssh

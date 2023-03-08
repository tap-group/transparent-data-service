# Transparent Data Service

This repository contains a reference implementation of TAP: Transparent and Privacy-Preserving Data Services. The code in this repository can be used to reproduce the experimental results from Section 6 of [the paper](https://www.usenix.org/system/files/sec23summer_125-reijsbergen-prepub.pdf).

WARNING: This is an academic prototype, and should not be used in applications without code review.

## How to run

### Dependencies

#### MySQL
Install MySQL and create a database with name `tap`.
Make sure the database is accessable with user `root` without a password.

#### Golang
Install golang 1.16 or later.

### Running the repo
Clone this repo.
Make sure gcc has been installed; if not, run the following (Linux)
```bash
sudo apt install build-essential
```
Download dependencies inside the repo directory.
```bash
go mod tidy
```

Run the experiments locally with a specific experiment flag.
```bash
go run . -experiment4
```

Run the experiments as server-client over network.
MySQL must run on the server side and this repo must be cloned on both server and client.

On the Server machine, build and run the service.
```bash
go build ./cmd/dbsvc
./dbsvc
```

On client side, run the experiments with `remote` flag.
```bash
go run . -remote
```
You can specify server url with `serverURL` flag. The default is `http://localhost:9045`.

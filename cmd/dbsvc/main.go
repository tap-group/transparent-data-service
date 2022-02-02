package main

import (
	"flag"

	"github.com/daniel-stud/anomalies/network"
	"github.com/daniel-stud/anomalies/server"
	"github.com/daniel-stud/anomalies/tables"
)

func main() {
	addr := flag.String("addr", ":9045", "server listen address")
	flag.Parse()

	server := new(server.Server)
	factory := new(tables.TableFactory)
	api := network.NewServerAPI(server, factory)

	api.Serve(*addr)
}

package main

import (
	"flag"

	"github.com/tap-group/tdsvc/network"
	"github.com/tap-group/tdsvc/server"
	"github.com/tap-group/tdsvc/tables"
)

func main() {
	addr := flag.String("addr", ":9045", "server listen address")
	flag.Parse()

	server := new(server.Server)
	factory := new(tables.TableFactory)
	api := network.NewServerAPI(server, factory)

	api.Serve(*addr)
}

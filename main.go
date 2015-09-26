package main

import (
	"flag"
	"utils"
	"web"
)

func main() {
	var port string
	flag.StringVar(&port, "port", "8212", "DAS server port number")
	var verbose bool
	flag.BoolVar(&verbose, "verbose", false, "Verbose mode of DAS server")
	flag.Parse()
	utils.VERBOSE = verbose
	web.Server(port)
}

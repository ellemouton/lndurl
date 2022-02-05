package main

import (
	"log"

	"github.com/ellemouton/lndurl"
	"github.com/lightninglabs/lndclient"
)

func main() {
	server, err := lndurl.NewServer(&lndurl.Config{
		Username:    "elle",
		Protocol:    "http",
		Host:        "localhost",
		Port:        8080,
		LndAddr:     "localhost:10011",
		Network:     lndclient.NetworkRegtest,
		MacaroonDir: "/Users/elle/LL/dev-resources/docker-regtest/mounts/regtest/alice",
		TLSPath:     "/Users/elle/LL/dev-resources/docker-regtest/mounts/regtest/alice/tls.cert",
	})
	if err != nil {
		log.Fatalln(err)
	}

	log.Fatalln(server.Run())
}

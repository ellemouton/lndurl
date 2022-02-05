package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/lightninglabs/lndclient"

	"github.com/urfave/cli/v2"
)

func main() {
	app := cli.NewApp()

	app.Name = "lndurl-client"
	app.Usage = "Cli for lndurl-client"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:  "host",
			Value: "localhost:10013",
			Usage: "lnd instance rpc address",
		},
		&cli.StringFlag{
			Name:  "network",
			Value: "regtest",
			Usage: "the network",
		},
		&cli.StringFlag{
			Name:  "macpath",
			Value: "/Users/elle/LL/dev-resources/docker-regtest/mounts/regtest/charlie",
			Usage: "Path to lnd's mac dir",
		},
		&cli.StringFlag{
			Name:  "tlspath",
			Value: "/Users/elle/LL/dev-resources/docker-regtest/mounts/regtest/charlie/tls.cert",
			Usage: "Path to lnd's tls cert",
		},
	}
	app.Commands = append(app.Commands, payRequestCommand)

	err := app.Run(os.Args)
	if err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "[lndurl-client] %v\n", err)
	os.Exit(1)
}

func get(url string, out interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("GET request error: %w", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response body: %w", err)
	}
	defer resp.Body.Close()

	return json.Unmarshal(body, &out)
}

func getLND(ctx *cli.Context) (*lndclient.GrpcLndServices, error) {
	return lndclient.NewLndServices(&lndclient.LndServicesConfig{
		LndAddress:  ctx.String("host"),
		Network:     lndclient.Network(ctx.String("network")),
		MacaroonDir: ctx.String("macpath"),
		TLSPath:     ctx.String("tlspath"),
	})
}

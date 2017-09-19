package main

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/rancher/cni-driver/cnisetup"
	"github.com/urfave/cli"
)

// VERSION of the binary, that can be changed during build
var VERSION = "v0.0.0-dev"

func main() {
	app := cli.NewApp()
	app.Name = "rancher-cni-driver"
	app.Version = VERSION
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "metadata-address",
			Usage:  "metadata address to use",
			EnvVar: "RANCHER_METADATA_ADDRESS",
			Value:  cnisetup.DefaultMetadataAddress,
		},
		cli.BoolFlag{
			Name:  "debug",
			Usage: "Turn on debug logging",
		},
	}

	app.Action = run
	app.Run(os.Args)
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		log.SetLevel(log.DebugLevel)
	}

	if err := cnisetup.Do(c.String("metadata-address")); err != nil {
		log.Errorf("failed to setup CNI: %v", err)
		os.Exit(1)
	}

	return nil
}

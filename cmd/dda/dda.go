// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

/*
Runs a Data Distribution Agent (DDA) as a sidecar.

For usage details, run dda with the command line flag -h.
*/
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda"
	"github.com/coatyio/dda/plog"
)

// Injected with -ldflags variable main.version
var version = "devel"

func main() {
	var configFile string
	var help bool
	var nolog bool

	flag.Usage = usage
	flag.StringVar(&configFile, "c", "", "Path to DDA configuration file in YAML format")
	flag.BoolVar(&help, "h", false, "Show usage information")
	flag.BoolVar(&nolog, "n", false, "Disable diagnostic output")
	flag.Parse()

	if flag.Arg(0) == "version" {
		fmt.Println(versionInfo())
		os.Exit(0)
	}
	if flag.NArg() > 0 {
		usage()
		os.Exit(2)
	}
	if help {
		usage()
		os.Exit(0)
	}
	if nolog {
		plog.Disable()
	}

	plog.Println(versionInfo())

	cfgFile := lookupConfigFile(configFile)
	cfg, err := config.ReadConfig(cfgFile)
	if err != nil {
		plog.Fatalf("Error reading DDA configuration file %s: %+v", cfgFile, err)
	} else {
		plog.Printf("config: %+v\n", *cfg)
	}

	signaled := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(signaled)
		sig := <-sigCh
		plog.Printf("DDA received signal %v", sig)
	}()

	var agent *dda.Dda
	agentDone := make(chan struct{})
	go func() {
		agt, err := dda.New(cfg)
		if err != nil {
			plog.Fatalf("DDA couldn't be created: %v", err)
		}
		err = agt.Open(0)
		if err != nil {
			plog.Fatalf("DDA couldn't be opened: %v", err)
		}
		agent = agt
		close(agentDone)
	}()

	select {
	case <-signaled:
	case <-agentDone:
		defer agent.Close()
		<-signaled
	}
}

func usage() {
	fmt.Printf(`Runs a Data Distribution Agent (DDA) as a sidecar.

For more information, examples, and documentation: https://github.com/coatyio/dda

A DDA is configured by a YAML configuration file which is looked up in the
following order:

1. on the command line using flag -c
2. in environment variable $DDA_CONFIGFILE
3. in a file "dda.yaml" located in the program's working directory

Usage:
  dda [-c configFile] [-h] [-n] [command]

Commands:
  version    Shows version information

Flags:
`)
	flag.PrintDefaults()
}

func lookupConfigFile(configFile string) string {
	if configFile == "" {
		configFile = os.Getenv("DDA_CONFIGFILE")
	}
	if configFile == "" {
		configFile = "dda.yaml"
	}
	return configFile
}

func versionInfo() string {
	return fmt.Sprintf("dda %s %s/%s %s", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
}

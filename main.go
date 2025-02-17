/*
Copyright (C)  2018 Yahoo Japan Corporation Athenz team.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/AthenZ/athenz-client-sidecar/v2/config"
	"github.com/AthenZ/athenz-client-sidecar/v2/usecase"
	"github.com/kpango/glg"
	"github.com/pkg/errors"
)

// Version is set by the build command via LDFLAGS
var Version string

type params struct {
	configFilePath string
	showVersion    bool
}

func parseParams() (*params, error) {
	p := new(params)
	f := flag.NewFlagSet(filepath.Base(os.Args[0]), flag.ContinueOnError)
	f.StringVar(&p.configFilePath,
		"f",
		"/etc/athenz/client/config.yaml",
		"client config yaml file path")
	f.BoolVar(&p.showVersion,
		"version",
		false,
		"show athenz-client-sidecar version")

	err := f.Parse(os.Args[1:])
	if err != nil {
		return nil, errors.Wrap(err, "Parse Failed")
	}

	return p, nil
}

func run(cfg config.Config) []error {
	g := glg.Get().SetMode(glg.NONE)

	switch cfg.Log.Level {
	case "":
		// disable logging
	case "fatal":
		g = g.SetLevelMode(glg.FATAL, glg.STD)
	case "error":
		g = g.SetLevelMode(glg.FATAL, glg.STD).
			SetLevelMode(glg.ERR, glg.STD)
	case "warn":
		g = g.SetLevelMode(glg.FATAL, glg.STD).
			SetLevelMode(glg.ERR, glg.STD).
			SetLevelMode(glg.WARN, glg.STD)
	case "info":
		g = g.SetLevelMode(glg.FATAL, glg.STD).
			SetLevelMode(glg.ERR, glg.STD).
			SetLevelMode(glg.WARN, glg.STD).
			SetLevelMode(glg.INFO, glg.STD)
	case "debug":
		g = g.SetLevelMode(glg.FATAL, glg.STD).
			SetLevelMode(glg.ERR, glg.STD).
			SetLevelMode(glg.WARN, glg.STD).
			SetLevelMode(glg.INFO, glg.STD).
			SetLevelMode(glg.DEBG, glg.STD)
	default:
		return []error{errors.New("invalid log level")}
	}

	if !cfg.Log.Color {
		g.DisableColor()
	}

	daemon, err := usecase.New(cfg)
	if err != nil {
		return []error{errors.Wrap(err, "tenant error")}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ech := daemon.Start(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	isSignal := false
	for {
		select {
		case sig := <-sigCh:
			glg.Infof("Athenz client sidecar received signal: %v", sig)
			isSignal = true
			cancel()
			glg.Warn("Athenz client sidecar shutdown...")
		case errs := <-ech:
			if !isSignal || len(errs) != 1 || errs[0] != ctx.Err() {
				return errs
			}
			return nil
		}
	}
}

func main() {
	defer func() {
		if err := recover(); err != nil {
			if _, ok := err.(runtime.Error); ok {
				panic(err)
			}
			glg.Error(err)
		}
	}()

	p, err := parseParams()
	if err != nil {
		glg.Fatal(err)
		return
	}

	if p.showVersion {
		err := glg.Infof("athenz-client-sidecar version -> %s", getVersion())
		if err != nil {
			glg.Fatal(err)
		}
		err = glg.Infof("athenz-client-sidecar config version -> %s", config.GetVersion())
		if err != nil {
			glg.Fatal(err)
		}
		return
	}

	cfg, err := config.New(p.configFilePath)
	if err != nil {
		glg.Fatal(err)
		return
	}

	if cfg.Version != config.GetVersion() {
		glg.Fatal(errors.New("invalid Athenz client proxy configuration version"))
		return
	}

	errs := run(*cfg)
	if len(errs) > 0 {
		glg.Fatalf("%+v", errs)
		return
	}
	glg.Info("Athenz client sidecar shutdown success")
	os.Exit(1)
}

func getVersion() string {
	if Version == "" {
		return "development version"
	}
	return Version
}

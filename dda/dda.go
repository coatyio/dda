// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package dda provides a ready-to-use Data Distribution Agent (DDA).
package dda

import (
	"fmt"
	"time"

	"github.com/coatyio/dda/apis"
	"github.com/coatyio/dda/apis/grpc"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/plog"
	"github.com/coatyio/dda/services/com"
	comapi "github.com/coatyio/dda/services/com/api"
	"github.com/coatyio/dda/services/state"
	stateapi "github.com/coatyio/dda/services/state/api"
	"github.com/coatyio/dda/services/store"
	storeapi "github.com/coatyio/dda/services/store/api"
)

// comApi is a non-exposed type alias for the communication API interface.
type comApi = comapi.Api

// storeApi is a non-exposed type alias for the local storage API interface.
type storeApi = storeapi.Api

// storeApi is a non-exposed type alias for the local storage API interface.
type stateApi = stateapi.Api

// Dda represents a Data Distribution Agent with peripheral services and public
// client APIs. It must be created with New() to ensure that all services and
// APIs are correctly initialized.
type Dda struct {
	cfg      *config.Config // agent configuration
	comApi                  // Communication API
	storeApi                // Local storage API
	stateApi                // State synchronization API

	grpcServer apis.ApiServer
}

// New creates a *Dda structure with DDA services and APIs initialized from the
// given configuration. An error is returned if the given configuration is
// invalid or if one of the DDA services or APIs cannot be initialized.
//
// To start the initialized DDA services and APIs invoke Open on the returned
// *Dda structure.
func New(cfg *config.Config) (*Dda, error) {
	if err := cfg.Verify(); err != nil {
		return nil, err
	}

	config := *cfg // copy to not mutate passed in config
	dda := Dda{cfg: &config}

	if !config.Services.Com.Disabled {
		comApi, err := com.New(config.Services.Com.Protocol)
		if err != nil {
			return nil, err
		} else {
			dda.comApi = *comApi
		}
	}

	if !config.Services.Store.Disabled {
		storeApi, err := store.New(config.Services.Store.Engine)
		if err != nil {
			return nil, err
		} else {
			dda.storeApi = *storeApi
		}
	}

	if !config.Services.State.Disabled {
		if config.Services.Com.Disabled {
			return nil, fmt.Errorf("Dda cannot be created: state service requires com service to be enabled")
		}
		stateApi, err := state.New(config.Services.State.Protocol)
		if err != nil {
			return nil, err
		} else {
			dda.stateApi = *stateApi
		}
	}

	if !config.Apis.Grpc.Disabled {
		dda.grpcServer = grpc.New(dda.comApi, dda.storeApi, dda.stateApi)
	}

	plog.Printf("Created DDA %+v", dda.Identity())

	return &dda, nil
}

// Identity gets the Identity of the DDA.
func (d *Dda) Identity() config.Identity {
	return d.cfg.Identity
}

// Open starts all configured DDA services and APIs and blocks waiting for them
// to be ready for use. An error is returned if some DDA services or APIs cannot
// be started, or if the given timeout elapses before setup of a single service
// or API completes. Specify a zero timeout to disable preliminary timeout
// behavior.
func (d *Dda) Open(timeout time.Duration) error {
	if d.comApi != nil {
		if err := <-d.comApi.Open(d.cfg, timeout); err != nil {
			return err
		}
	}

	if d.storeApi != nil {
		if err := d.storeApi.Open(d.cfg); err != nil {
			return err
		}
	}

	if d.stateApi != nil {
		if err := d.stateApi.Open(d.cfg, d.comApi); err != nil {
			return err
		}
	}

	if d.grpcServer != nil {
		if err := d.grpcServer.Open(d.cfg); err != nil {
			return err
		}
	}

	plog.Printf("Opened DDA %+v", d.Identity())

	return nil
}

// Close synchronously shuts down all configured DDA services and APIs
// gracefully and releases associated resources.
func (d *Dda) Close() {
	if d.grpcServer != nil {
		d.grpcServer.Close()
	}

	if d.stateApi != nil {
		d.stateApi.Close()
	}

	if d.storeApi != nil {
		d.storeApi.Close()
	}

	if d.comApi != nil { // finally
		<-d.comApi.Close()
	}

	plog.Printf("Closed DDA %+v", d.Identity())
}

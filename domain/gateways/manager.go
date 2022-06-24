package gateways

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"gitlab.com/moderntoken/gateways/core"
	"gitlab.com/moderntoken/gateways/gws/binance"
)

type Manager interface {
	Gateway(name string) (core.Gateway, bool)
	Status() map[string]core.GatewayStatus
}

// manager holds and controls gateways (core.Gateway implementations)
type manager struct {
	gateways map[string]core.Gateway
}

func NewManager() *manager {
	m := &manager{
		gateways: make(map[string]core.Gateway),
	}

	// Initialize gateways
	for _, gw := range []core.Gateway{
		binance.New(binance.Config{Mode: binance.Prod}),
	} {
		m.gateways[gw.Name()] = gw
	}

	return m
}

// Start starts all gateways concurrently and blocks waiting for all
func (m *manager) Start(ctx context.Context) {
	// Start gateways in parallel
	var wg sync.WaitGroup
	for _, gw := range m.gateways {
		wg.Add(1)
		go func(gw core.Gateway) {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				err := gw.Start()
				if err == nil {
					log.Info().Msgf("Gateway %q is ready", gw.Name())
					return
				}
				log.Error().Stack().Err(err).Msgf("Failed to start %s", gw.Name())
			}
		}(gw)
	}

	wg.Wait()
}

// Gateway returns gateway by its name
func (m *manager) Gateway(name string) (core.Gateway, bool) {
	gw, ok := m.gateways[name]
	return gw, ok
}

// Status aggregates all gateways statuses
func (m *manager) Status() map[string]core.GatewayStatus {
	res := make(map[string]core.GatewayStatus, len(m.gateways))
	for _, gateway := range m.gateways {
		res[gateway.Name()] = gateway.Status()
	}
	return res
}

// Stop all gateways
func (m *manager) Stop() {
	for _, gateway := range m.gateways {
		gateway.Stop()
	}
}

package adr

import (
	"github.com/brocaar/chirpstack-network-server/v3/adr"
)

// MaxTXPowerHandler is the default ADR algorithm, except that it keeps the
// device at max TX power. The default algorithm uses the max SNR of the
// uplink history, so a single good uplink of a battery water meter (8 h
// uplink interval) lowered its TX power by up to 14 dB, and raising it again
// needed ~20 uplinks.
type MaxTXPowerHandler struct {
	DefaultHandler
}

// ID returns the ID.
func (h *MaxTXPowerHandler) ID() (string, error) {
	return "max_tx_power", nil
}

// Name returns the name.
func (h *MaxTXPowerHandler) Name() (string, error) {
	return "Default ADR algorithm, always max TX power (Sensorico)", nil
}

// Handle handles the ADR request.
func (h *MaxTXPowerHandler) Handle(req adr.HandleRequest) (adr.HandleResponse, error) {
	if !req.ADR {
		return h.DefaultHandler.Handle(req)
	}

	req.MaxTxPowerIndex = 0
	resp, err := h.DefaultHandler.Handle(req)
	if err != nil {
		return resp, err
	}

	resp.TxPowerIndex = 0
	return resp, nil
}

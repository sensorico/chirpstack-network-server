package adr

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brocaar/chirpstack-network-server/v3/adr"
	"github.com/brocaar/chirpstack-network-server/v3/internal/band"
	"github.com/brocaar/chirpstack-network-server/v3/internal/test"
)

func TestMaxTXPowerHandler(t *testing.T) {
	band.Setup(test.GetConfig())
	h := &MaxTXPowerHandler{}

	tests := []struct {
		name     string
		request  adr.HandleRequest
		response adr.HandleResponse
	}{
		{
			name: "adr disabled, keep current values",
			request: adr.HandleRequest{
				ADR:             false,
				DR:              3,
				TxPowerIndex:    7,
				NbTrans:         1,
				MaxDR:           3,
				MaxTxPowerIndex: 7,
			},
			response: adr.HandleResponse{
				DR:           3,
				TxPowerIndex: 7,
				NbTrans:      1,
			},
		},
		{
			name: "max dr reached with spare margin, keep max tx-power",
			request: adr.HandleRequest{
				ADR:              true,
				DR:               3,
				TxPowerIndex:     0,
				NbTrans:          1,
				MaxDR:            3,
				MaxTxPowerIndex:  7,
				RequiredSNRForDR: -12.5,
				UplinkHistory:    []adr.UplinkMetaData{{MaxSNR: 6.5}},
			},
			response: adr.HandleResponse{
				DR:           3,
				TxPowerIndex: 0,
				NbTrans:      1,
			},
		},
		{
			name: "lowered tx-power, restore max tx-power",
			request: adr.HandleRequest{
				ADR:              true,
				DR:               3,
				TxPowerIndex:     7,
				NbTrans:          1,
				MaxDR:            3,
				MaxTxPowerIndex:  7,
				RequiredSNRForDR: -12.5,
				UplinkHistory:    []adr.UplinkMetaData{{MaxSNR: -9, TXPowerIndex: 7}},
			},
			response: adr.HandleResponse{
				DR:           3,
				TxPowerIndex: 0,
				NbTrans:      1,
			},
		},
		{
			name: "spare margin below max dr, increase dr like default",
			request: adr.HandleRequest{
				ADR:              true,
				DR:               0,
				TxPowerIndex:     0,
				NbTrans:          1,
				MaxDR:            3,
				MaxTxPowerIndex:  7,
				RequiredSNRForDR: -20,
				UplinkHistory:    []adr.UplinkMetaData{{MaxSNR: -15}},
			},
			response: adr.HandleResponse{
				DR:           1,
				TxPowerIndex: 0,
				NbTrans:      1,
			},
		},
	}

	for _, tst := range tests {
		t.Run(tst.name, func(t *testing.T) {
			assert := require.New(t)

			resp, err := h.Handle(tst.request)
			assert.NoError(err)
			assert.Equal(tst.response, resp)
		})
	}
}

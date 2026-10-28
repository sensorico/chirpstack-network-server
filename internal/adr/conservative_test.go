package adr

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/brocaar/chirpstack-network-server/v3/adr"
	"github.com/brocaar/chirpstack-network-server/v3/internal/band"
	"github.com/brocaar/chirpstack-network-server/v3/internal/test"
)

func uplinkHistory(snrs []float32, txPowerIndex int) []adr.UplinkMetaData {
	var out []adr.UplinkMetaData
	for i, snr := range snrs {
		out = append(out, adr.UplinkMetaData{FCnt: uint32(i), MaxSNR: snr, TXPowerIndex: txPowerIndex, GatewayCount: 1})
	}
	return out
}

func repeatSNR(snr float32, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = snr
	}
	return out
}

func TestConservativeHandler(t *testing.T) {
	band.Setup(test.GetConfig())
	h := &ConservativeHandler{}

	outlier := append(repeatSNR(-5, 19), 10)
	lossy := uplinkHistory(repeatSNR(8, 20), 0)
	lossy[19].FCnt = 21

	tests := []struct {
		name     string
		request  adr.HandleRequest
		response adr.HandleResponse
	}{
		{
			name:     "adr disabled, keep current values",
			request:  adr.HandleRequest{ADR: false, DR: 3, TxPowerIndex: 7, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 7, NbTrans: 1},
		},
		{
			name: "short history, restore max tx-power and keep dr",
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 7, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(8, 5), 7)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 0, NbTrans: 1},
		},
		{
			name: "max dr exceeded, decrease dr",
			request: adr.HandleRequest{ADR: true, DR: 5, TxPowerIndex: 0, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(-20, 3), 0)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 0, NbTrans: 1},
		},
		{
			name: "single good uplink among weak ones, keep max tx-power",
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 0, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(outlier, 0)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 0, NbTrans: 1},
		},
		{
			name: "consistently strong at max dr, lower tx-power within worst-case margin",
			// worst -12.5 dB required at SF9 + 10 dB margin = 10.5 dB spare -> 10 dB (index 5)
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 0, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(8, 20), 0)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 5, NbTrans: 1},
		},
		{
			name: "history at lowered tx-power is normalised to max tx-power",
			// -4 dB at index 7 (-14 dB) = +10 dB at max power -> 12.5 dB spare -> index 6
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 7, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(-4, 20), 7)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 6, NbTrans: 1},
		},
		{
			name: "worst case margin shrank, raise tx-power",
			// -12 dB at index 7 = +2 dB at max power -> 4.5 dB spare -> index 2
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 7, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(-12, 20), 7)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 2, NbTrans: 1},
		},
		{
			name: "packet loss, keep max tx-power",
			request: adr.HandleRequest{ADR: true, DR: 3, TxPowerIndex: 5, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: lossy},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 0, NbTrans: 2},
		},
		{
			name: "six consistent uplinks, increase dr up to max dr at max tx-power",
			// worst -2 dB: SF10 -15 / SF9 -12.5 still leave >= 10 dB margin
			request: adr.HandleRequest{ADR: true, DR: 0, TxPowerIndex: 0, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(repeatSNR(-2, 6), 0)},
			response: adr.HandleResponse{DR: 3, TxPowerIndex: 0, NbTrans: 1},
		},
		{
			name: "worst case too weak for a faster dr, keep dr",
			request: adr.HandleRequest{ADR: true, DR: 0, TxPowerIndex: 0, NbTrans: 1, MaxDR: 3, MaxTxPowerIndex: 7,
				InstallationMargin: 10, UplinkHistory: uplinkHistory(append(repeatSNR(-2, 5), -9), 0)},
			response: adr.HandleResponse{DR: 0, TxPowerIndex: 0, NbTrans: 1},
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

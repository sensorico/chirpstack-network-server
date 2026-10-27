package adr

import (
	"fmt"

	"github.com/brocaar/chirpstack-network-server/v3/adr"
	"github.com/brocaar/chirpstack-network-server/v3/internal/band"
	"github.com/brocaar/chirpstack-network-server/v3/internal/config"
	loraband "github.com/brocaar/lorawan/band"
)

const (
	conservativeMinHistoryForDR      = 6
	conservativeMinHistoryForTXPower = 20
	conservativeMaxPacketLoss        = 5
)

// ConservativeHandler judges the link by the worst SNR of the uplink history
// (normalised to max TX power) instead of the best one. The default algorithm
// lowered battery water meters (8 h uplink interval) by up to 14 dB after a
// single good uplink; this one only lowers TX power when all 20 recent
// uplinks keep the installation margin, and returns to max TX power on
// packet loss or a short history.
type ConservativeHandler struct {
	DefaultHandler
}

// ID returns the ID.
func (h *ConservativeHandler) ID() (string, error) {
	return "conservative", nil
}

// Name returns the name.
func (h *ConservativeHandler) Name() (string, error) {
	return "Worst-SNR ADR algorithm, lowers TX power only with proven margin (Sensorico)", nil
}

// Handle handles the ADR request.
func (h *ConservativeHandler) Handle(req adr.HandleRequest) (adr.HandleResponse, error) {
	resp := adr.HandleResponse{
		DR:           req.DR,
		TxPowerIndex: req.TxPowerIndex,
		NbTrans:      req.NbTrans,
	}

	if !req.ADR {
		return resp, nil
	}

	maxDR, err := maxLoRaDR(req.MaxDR)
	if err != nil {
		return resp, err
	}
	if resp.DR > maxDR {
		resp.DR = maxDR
	}

	resp.TxPowerIndex = 0
	resp.NbTrans = h.getNbTrans(req.NbTrans, h.getPacketLossPercentage(req))

	if len(req.UplinkHistory) < conservativeMinHistoryForDR || packetLossPercentage(req.UplinkHistory) >= conservativeMaxPacketLoss {
		return resp, nil
	}

	worstSNR, err := worstSNRAtMaxTXPower(req.UplinkHistory)
	if err != nil {
		return resp, err
	}

	for resp.DR < maxDR {
		margin, err := snrMargin(worstSNR, resp.DR+1, req.InstallationMargin)
		if err != nil {
			return resp, err
		}
		if margin < 0 {
			break
		}
		resp.DR++
	}

	if len(req.UplinkHistory) < conservativeMinHistoryForTXPower {
		return resp, nil
	}

	spare, err := snrMargin(worstSNR, resp.DR, req.InstallationMargin)
	if err != nil {
		return resp, err
	}

	for i := 1; i <= req.MaxTxPowerIndex; i++ {
		offset, err := band.Band().GetTXPowerOffset(i)
		if err != nil || float32(-offset) > spare {
			break
		}
		resp.TxPowerIndex = i
	}

	return resp, nil
}

// maxLoRaDR returns the given max DR, reduced to the highest enabled LoRa
// 125 kHz data-rate.
func maxLoRaDR(maxDR int) (int, error) {
	maxLoRaDR := 0
	for _, i := range band.Band().GetEnabledUplinkDataRates() {
		dr, err := band.Band().GetDataRate(i)
		if err != nil {
			return 0, err
		}
		if dr.Modulation == loraband.LoRaModulation && dr.Bandwidth == 125 {
			maxLoRaDR = i
		}
	}

	if maxDR > maxLoRaDR {
		return maxLoRaDR, nil
	}
	return maxDR, nil
}

func worstSNRAtMaxTXPower(history []adr.UplinkMetaData) (float32, error) {
	worst := float32(999)
	for _, uh := range history {
		offset, err := band.Band().GetTXPowerOffset(uh.TXPowerIndex)
		if err != nil {
			return 0, err
		}
		if snr := uh.MaxSNR - float32(offset); snr < worst {
			worst = snr
		}
	}
	return worst, nil
}

func snrMargin(snr float32, dr int, installationMargin float32) (float32, error) {
	dataRate, err := band.Band().GetDataRate(dr)
	if err != nil {
		return 0, err
	}
	required, ok := config.SpreadFactorToRequiredSNRTable[dataRate.SpreadFactor]
	if !ok {
		return 0, fmt.Errorf("no required snr for spread-factor %d", dataRate.SpreadFactor)
	}
	return snr - float32(required) - installationMargin, nil
}

// packetLossPercentage is the default handler's loss calculation without its
// 20-uplink minimum, so losses also block a data-rate increase.
func packetLossPercentage(history []adr.UplinkMetaData) float32 {
	if len(history) < 2 {
		return 0
	}

	var lost uint32
	for i := 1; i < len(history); i++ {
		if history[i].FCnt > history[i-1].FCnt {
			lost += history[i].FCnt - history[i-1].FCnt - 1
		}
	}
	return float32(lost) / float32(len(history)) * 100
}

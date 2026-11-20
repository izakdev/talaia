package qnap

import (
	"slices"

	"github.com/izakdev/talaia/internal/integrations"
)

// Un volumen por encima de este uso se marca en ámbar.
const fullPercent = 90

func tempTone(c, warn, err int) integrations.Tone {
	switch {
	case c >= err:
		return integrations.ToneBad
	case c >= warn:
		return integrations.ToneWarn
	}
	return integrations.ToneGood
}

func fill(v Volume) float64 {
	if v.TotalBytes <= 0 {
		return 0
	}
	return float64(v.TotalBytes-v.FreeBytes) / float64(v.TotalBytes) * 100
}

// State devuelve el estado del NAS para el panel y los avisos. El primer
// problema de la lista da la etiqueta; el orden es el de gravedad.
func (d Data) State() integrations.State {
	temps := []integrations.Tone{}
	for _, t := range []*Temperature{d.CPUTemperature, d.SystemTemp} {
		if t != nil {
			temps = append(temps, tempTone(t.C, t.WarnC, t.ErrC))
		}
	}
	for _, disk := range d.Disks {
		if disk.TempC != nil {
			temps = append(temps, tempTone(*disk.TempC, d.DiskTempWarnC, d.DiskTempErrC))
		}
	}
	disk := func(health string) bool {
		return slices.ContainsFunc(d.Disks, func(x Disk) bool { return x.Health == health })
	}
	storage := func(status string) bool {
		return d.Storage == status || slices.ContainsFunc(d.Volumes, func(v Volume) bool { return v.Status == status })
	}
	checks := []struct {
		hit  bool
		tone integrations.Tone
		key  string
	}{
		{disk("error"), integrations.ToneBad, "qnap.state.diskError"},
		{storage("degraded"), integrations.ToneBad, "qnap.state.degraded"},
		{storage("error"), integrations.ToneBad, "qnap.state.storageError"},
		{slices.Contains(temps, integrations.ToneBad), integrations.ToneBad, "qnap.state.hot"},
		{disk("warning"), integrations.ToneWarn, "qnap.state.smartWarning"},
		{storage("syncing"), integrations.ToneWarn, "qnap.state.syncing"},
		{storage("warning"), integrations.ToneWarn, "qnap.state.storageWarning"},
		{slices.ContainsFunc(d.Volumes, func(v Volume) bool { return fill(v) >= fullPercent }), integrations.ToneWarn, "qnap.state.full"},
		{slices.Contains(temps, integrations.ToneWarn), integrations.ToneWarn, "qnap.state.warm"},
		{d.FanFailed, integrations.ToneWarn, "qnap.state.fan"},
	}
	for _, c := range checks {
		if c.hit {
			return integrations.State{Tone: c.tone, Key: c.key}
		}
	}
	if len(d.Disks) == 0 {
		return integrations.State{Tone: integrations.ToneMuted, Key: "qnap.state.noDisks"}
	}
	return integrations.State{Tone: integrations.ToneGood, Key: "qnap.state.ok"}
}

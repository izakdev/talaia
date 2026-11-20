package qnap

import (
	"testing"

	"github.com/izakdev/talaia/internal/integrations"
	st "github.com/izakdev/talaia/internal/integrations/statetest"
)

func s(tone integrations.Tone, key string) integrations.State {
	return integrations.State{Tone: tone, Key: key}
}

const (
	good  = integrations.ToneGood
	warn  = integrations.ToneWarn
	bad   = integrations.ToneBad
	muted = integrations.ToneMuted
)

func ok() Data {
	t := 40
	return Data{
		Storage: "ready", DiskTempWarnC: 50, DiskTempErrC: 60,
		CPUTemperature: &Temperature{C: 50, WarnC: 70, ErrC: 85},
		Disks:          []Disk{{Health: "good", TempC: &t}},
		Volumes:        []Volume{{Status: "ready", TotalBytes: 100, FreeBytes: 50}},
	}
}

func with(f func(*Data)) Data {
	d := ok()
	f(&d)
	return d
}

func TestState(t *testing.T) {
	hot, warm := 61, 55
	st.Run(t, []st.Case{
		st.C("disco con fallos y volumen lleno", with(func(d *Data) { d.Disks[0].Health = "error"; d.Volumes[0].FreeBytes = 10 }), s(bad, "qnap.state.diskError")),
		st.C("almacenamiento degradado", with(func(d *Data) { d.Volumes[0].Status = "degraded" }), s(bad, "qnap.state.degraded")),
		st.C("almacenamiento con errores", with(func(d *Data) { d.Storage = "error" }), s(bad, "qnap.state.storageError")),
		st.C("disco a temperatura crítica", with(func(d *Data) { d.Disks[0].TempC = &hot }), s(bad, "qnap.state.hot")),
		st.C("cpu a temperatura crítica", with(func(d *Data) { d.CPUTemperature.C = 85 }), s(bad, "qnap.state.hot")),
		st.C("aviso smart", with(func(d *Data) { d.Disks[0].Health = "warning" }), s(warn, "qnap.state.smartWarning")),
		st.C("sincronizando", with(func(d *Data) { d.Volumes[0].Status = "syncing" }), s(warn, "qnap.state.syncing")),
		st.C("almacenamiento con avisos", with(func(d *Data) { d.Storage = "warning" }), s(warn, "qnap.state.storageWarning")),
		st.C("volumen al 90 %", with(func(d *Data) { d.Volumes[0].FreeBytes = 10 }), s(warn, "qnap.state.full")),
		st.C("disco templado", with(func(d *Data) { d.Disks[0].TempC = &warm }), s(warn, "qnap.state.warm")),
		st.C("sistema templado", with(func(d *Data) { d.SystemTemp = &Temperature{C: 70, WarnC: 70, ErrC: 80} }), s(warn, "qnap.state.warm")),
		st.C("ventilador", with(func(d *Data) { d.FanFailed = true }), s(warn, "qnap.state.fan")),
		st.C("sin discos", with(func(d *Data) { d.Disks = nil }), s(muted, "qnap.state.noDisks")),
		st.C("disco sin temperatura", with(func(d *Data) { d.Disks[0].TempC = nil }), s(good, "qnap.state.ok")),
		st.C("volumen sin tamaño", with(func(d *Data) { d.Volumes[0].TotalBytes = 0 }), s(good, "qnap.state.ok")),
		st.C("todo bien", ok(), s(good, "qnap.state.ok")),
	})
}

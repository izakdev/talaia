package notify

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/secrets"
	"github.com/izakdev/talaia/internal/store"
)

// source simula el recolector.
type source struct {
	mu    sync.Mutex
	snaps map[string]collector.Snapshot
}

func (s *source) Snapshots() []collector.Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []collector.Snapshot{}
	for _, snap := range s.snaps {
		out = append(out, snap)
	}
	return out
}

func (s *source) Subscribe() (<-chan struct{}, func(), bool) {
	return make(chan struct{}), func() {}, true
}

// sent registra lo que llega a cada canal.
type sent struct {
	mu   sync.Mutex
	msgs map[string][]Message
	// fail decide el resultado de cada envío por canal y número de intento.
	fail func(channel string, attempt int) error
	hits map[string]int
}

type fakeSender struct {
	s       *sent
	channel string
}

func (f fakeSender) Send(_ context.Context, m Message) error {
	f.s.mu.Lock()
	f.s.hits[f.channel]++
	attempt := f.s.hits[f.channel]
	fail := f.s.fail
	f.s.mu.Unlock()
	if fail != nil {
		if err := fail(f.channel, attempt); err != nil {
			return err
		}
	}
	f.s.mu.Lock()
	defer f.s.mu.Unlock()
	f.s.msgs[f.channel] = append(f.s.msgs[f.channel], m)
	return nil
}

func (s *sent) count(channel string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.msgs[channel])
}

func (s *sent) last(t *testing.T, channel string) Message {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.msgs[channel]) == 0 {
		t.Fatalf("no ha llegado nada a %s", channel)
	}
	return s.msgs[channel][len(s.msgs[channel])-1]
}

type harness struct {
	st    *store.Store
	src   *source
	n     *Notifier
	sent  *sent
	clock time.Time
	path  string
	sleep func()
}

var key = bytes.Repeat([]byte{5}, 32)

func open(t *testing.T, path string, k []byte) *store.Store {
	t.Helper()
	box, _ := secrets.New(k)
	st, err := store.Open(path, box)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	path := filepath.Join(t.TempDir(), "t.db")
	h := &harness{st: open(t, path, key), src: &source{snaps: map[string]collector.Snapshot{}}, clock: now, path: path,
		sent: &sent{msgs: map[string][]Message{}, hits: map[string]int{}}}
	h.n = h.notifier(h.st)
	return h
}

func (h *harness) notifier(st *store.Store) *Notifier {
	n := New(st, h.src, slog.New(slog.NewTextHandler(io.Discard, nil)))
	n.now = func() time.Time { return h.clock }
	n.newSender = func(ch store.Channel, _ string) (Sender, error) { return fakeSender{h.sent, ch.Name}, nil }
	n.sleep = func(ctx context.Context, _ time.Duration) error {
		if h.sleep != nil {
			h.sleep()
		}
		return ctx.Err()
	}
	return n
}

func (h *harness) channel(t *testing.T, name string) store.Channel {
	t.Helper()
	c, err := h.st.CreateChannel(context.Background(), store.Channel{Name: name, Type: "ntfy", Enabled: true}, `{"topic":"t"}`)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func (h *harness) service(t *testing.T, name string, notify bool) string {
	t.Helper()
	c, err := h.st.CreateConnection(context.Background(), store.Connection{Name: name, Type: "adguard", BaseURL: "http://x", IntervalSeconds: 30, Notify: notify}, "k")
	if err != nil {
		t.Fatal(err)
	}
	h.set(c.ID, name, integrations.ToneGood, "adguard.protectionOn")
	return c.ID
}

func (h *harness) set(id, name string, tone integrations.Tone, key string) {
	h.src.mu.Lock()
	defer h.src.mu.Unlock()
	h.src.snaps[id] = collector.Snapshot{ID: id, Name: name, Type: "adguard", State: integrations.State{Tone: tone, Key: key}}
}

// step avanza el reloj, evalúa y espera a que terminen los envíos.
func (h *harness) step(t *testing.T, d time.Duration) {
	t.Helper()
	h.clock = h.clock.Add(d)
	if err := h.n.evaluate(context.Background(), h.clock); err != nil {
		t.Fatal(err)
	}
	h.n.Wait()
}

const (
	good  = integrations.ToneGood
	warn  = integrations.ToneWarn
	bad   = integrations.ToneBad
	muted = integrations.ToneMuted
)

func TestNotifiesAfterDelay(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.step(t, 0)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.step(t, 119*time.Second)
	if h.sent.count("móvil") != 0 {
		t.Fatal("avisa antes del retardo")
	}
	h.step(t, time.Second)
	m := h.sent.last(t, "móvil")
	if m.Text != "🟠 Talaia · Necesita atención\nAdGuard: Protección desactivada" || h.sent.count("móvil") != 1 {
		t.Fatalf("mensaje = %q", m.Text)
	}
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 1 {
		t.Fatal("repite el aviso")
	}
	log, _ := h.st.ListLog(context.Background())
	if len(log) != 1 || log[0].Result != "ok" || log[0].Attempts != 1 || log[0].Channel != "móvil" || log[0].Services[0].Previous != "good" || log[0].Services[0].Tone != "warn" {
		t.Fatalf("registro = %+v", log)
	}
}

func TestFlapDoesNotNotify(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.step(t, 60*time.Second)
	h.set(id, "AdGuard", good, "adguard.protectionOn")
	h.step(t, 10*time.Second)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 10*time.Second)
	h.step(t, 100*time.Second)
	if h.sent.count("móvil") != 0 {
		t.Fatal("el parpadeo no reinicia el plazo")
	}
	h.step(t, 20*time.Second)
	if h.sent.count("móvil") != 1 {
		t.Fatal("no avisa tras 120 s estables")
	}
}

func TestColorChangeRestartsCandidate(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.step(t, 100*time.Second)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 30*time.Second)
	if h.sent.count("móvil") != 0 {
		t.Fatal("el rojo hereda el plazo del ámbar")
	}
	h.step(t, 89*time.Second)
	if h.sent.count("móvil") != 0 {
		t.Fatal("el rojo avisa antes de su propio plazo")
	}
	h.step(t, 31*time.Second)
	if m := h.sent.last(t, "móvil"); m.Worst != bad {
		t.Fatalf("mensaje = %+v", m)
	}
}

func TestMutedSkipped(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 60*time.Second)
	h.set(id, "AdGuard", muted, "status.checking")
	h.step(t, 30*time.Second)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 30*time.Second)
	if h.sent.count("móvil") != 1 {
		t.Fatal("un gris intermedio reinicia el plazo")
	}
	// Un gris largo tampoco avisa de nada ni cambia el color avisado.
	h.set(id, "AdGuard", muted, "status.checking")
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 1 {
		t.Fatal("el gris avisa")
	}
	if tones, _ := h.st.NotifiedTones(context.Background()); tones[id] != "bad" {
		t.Fatalf("tonos = %v", tones)
	}
}

func TestGroupsSimultaneous(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "móvil")
	a := h.service(t, "AdGuard", true)
	v := h.service(t, "VPN", true)
	h.set(a, "AdGuard", warn, "adguard.protectionOff")
	h.set(v, "VPN", bad, "adguard.dnsStopped")
	h.step(t, 0)
	m := h.sent.last(t, "móvil")
	if h.sent.count("móvil") != 1 || len(m.Changes) != 2 || m.Changes[0].Name != "VPN" {
		t.Fatalf("mensajes = %d, cambios = %+v", h.sent.count("móvil"), m.Changes)
	}
}

func TestSendsToEveryEnabledChannel(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0, NotifyLanguage: "en"})
	h.channel(t, "uno")
	h.channel(t, "dos")
	off := h.channel(t, "apagado")
	off.Enabled = false
	h.st.UpdateChannel(context.Background(), off, nil)
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	if h.sent.count("uno") != 1 || h.sent.count("dos") != 1 || h.sent.count("apagado") != 0 {
		t.Fatalf("envíos = %v", h.sent.msgs)
	}
	if m := h.sent.last(t, "uno"); m.Language != "en" {
		t.Fatalf("idioma = %q", m.Language)
	}
}

func TestRecovery(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	h.set(id, "AdGuard", good, "adguard.protectionOn")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	m := h.sent.last(t, "móvil")
	if h.sent.count("móvil") != 2 || m.Text != "🟢 Talaia · Vuelve a estar en orden\nAdGuard: Protección activa" || m.Changes[0].Previous != bad {
		t.Fatalf("mensaje = %+v", m)
	}
}

func TestSilencedUpdatesTone(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", false)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	if h.sent.count("móvil") != 0 {
		t.Fatal("un servicio silenciado avisa")
	}
	c, _ := h.st.GetConnection(context.Background(), id)
	c.Notify = true
	h.st.UpdateConnection(context.Background(), c, nil)
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 0 {
		t.Fatal("al reactivarlo llega un aviso atrasado")
	}
	if log, _ := h.st.ListLog(context.Background()); len(log) != 0 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestNoChannelsUpdatesTone(t *testing.T) {
	h := newHarness(t)
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	if tones, _ := h.st.NotifiedTones(context.Background()); tones[id] != "bad" {
		t.Fatalf("tonos = %v", tones)
	}
	h.channel(t, "móvil")
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 0 {
		t.Fatal("el primer canal recibe problemas antiguos")
	}
	if log, _ := h.st.ListLog(context.Background()); len(log) != 0 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestNotOnPanelIgnoredAndForgotten(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	if h.sent.count("móvil") != 1 {
		t.Fatal("no avisa")
	}
	if err := h.st.SavePanel(ctx, []store.Page{{ID: "inicio", Name: "Inicio", Widgets: []store.Widget{}}}); err != nil {
		t.Fatal(err)
	}
	h.set(id, "AdGuard", good, "adguard.protectionOn")
	h.step(t, 0)
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 1 {
		t.Fatal("avisa de un servicio que no está en el panel")
	}
	if tones, _ := h.st.NotifiedTones(ctx); len(tones) != 0 {
		t.Fatalf("no olvida el color al quitarlo del panel: %v", tones)
	}
}

func TestFirstStateAssumesGood(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	ok := h.service(t, "Bien", true)
	red := h.service(t, "Mal", true)
	h.set(red, "Mal", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	m := h.sent.last(t, "móvil")
	if h.sent.count("móvil") != 1 || len(m.Changes) != 1 || m.Changes[0].Name != "Mal" || m.Changes[0].Previous != good {
		t.Fatalf("mensaje = %+v", m)
	}
	_ = ok
}

func TestRestartDoesNotRepeat(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", bad, "adguard.dnsStopped")
	h.step(t, 0)
	h.step(t, 2*time.Minute)
	h.n = h.notifier(h.st)
	h.step(t, 0)
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 1 {
		t.Fatal("repite el aviso tras reiniciar")
	}
}

func TestRetryThenLog(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "móvil")
	h.sent.fail = func(_ string, attempt int) error {
		if attempt < 3 {
			return &integrations.Error{Code: integrations.CodeBadResponse, Err: errors.New("500")}
		}
		return nil
	}
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	log, _ := h.st.ListLog(context.Background())
	if h.sent.count("móvil") != 1 || len(log) != 1 || log[0].Result != "ok" || log[0].Attempts != 3 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestRejectedIsNotRetried(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "móvil")
	h.sent.fail = func(string, int) error {
		return &integrations.Error{Code: integrations.CodeUnauthorized, Err: errors.New("401")}
	}
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	log, _ := h.st.ListLog(context.Background())
	if len(log) != 1 || log[0].Result != "error" || log[0].Code != "unauthorized" || log[0].Attempts != 1 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestUnconfirmedIsNotRetried(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "móvil")
	h.sent.fail = func(string, int) error {
		return &integrations.Error{Code: CodeUnconfirmed, Err: errors.New("sin respuesta tras enviar")}
	}
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.step(t, 5*time.Minute)
	log, _ := h.st.ListLog(context.Background())
	if len(log) != 1 || log[0].Result != "error" || log[0].Code != CodeUnconfirmed || log[0].Attempts != 1 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestRetrySkipsDeletedChannel(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.st.SaveSettings(ctx, store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	ch := h.channel(t, "móvil")
	h.sent.fail = func(string, int) error {
		return &integrations.Error{Code: integrations.CodeUnreachable, Err: errors.New("caído")}
	}
	h.sleep = func() { _ = h.st.DeleteChannel(ctx, ch.ID) }
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	log, _ := h.st.ListLog(ctx)
	if h.sent.hits["móvil"] != 1 || len(log) != 1 || log[0].Code != "unreachable" || log[0].Attempts != 1 {
		t.Fatalf("intentos = %d, registro = %+v", h.sent.hits["móvil"], log)
	}
}

func TestUnreadableChannelSecret(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.st.SaveSettings(ctx, store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "móvil")
	id := h.service(t, "AdGuard", true)
	h.st.Close()
	// La misma base de datos con otra clave: el secreto no se puede descifrar.
	other := open(t, h.path, bytes.Repeat([]byte{6}, 32))
	if _, err := other.CreateChannel(ctx, store.Channel{Name: "bueno", Type: "ntfy", Enabled: true}, `{"topic":"t"}`); err != nil {
		t.Fatal(err)
	}
	h.st, h.n = other, h.notifier(other)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	log, _ := other.ListLog(ctx)
	codes := map[string]string{}
	for _, e := range log {
		codes[e.Channel] = e.Code
	}
	if codes["móvil"] != "secret_unreadable" || codes["bueno"] != "" || h.sent.count("bueno") != 1 {
		t.Fatalf("registro = %+v", log)
	}
}

func TestSlowChannelDoesNotBlockOthers(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.st.SaveSettings(ctx, store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 0})
	h.channel(t, "lento")
	h.channel(t, "rápido")
	release := make(chan struct{})
	h.sent.fail = func(channel string, _ int) error {
		if channel == "lento" {
			<-release
		}
		return nil
	}
	id := h.service(t, "AdGuard", true)
	h.set(id, "AdGuard", warn, "adguard.protectionOff")
	done := make(chan struct{})
	go func() {
		_ = h.n.evaluate(ctx, h.clock)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("evaluate espera al canal lento")
	}
	deadline := time.Now().Add(2 * time.Second)
	for h.sent.count("rápido") == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if h.sent.count("rápido") != 1 {
		t.Fatal("el canal rápido espera al lento")
	}
	close(release)
	h.n.Wait()
}

func TestSendTest(t *testing.T) {
	h := newHarness(t)
	h.st.SaveSettings(context.Background(), store.Settings{Theme: "auto", Accent: "amber", NotifyDelaySeconds: 120, NotifyLanguage: "en"})
	entry := h.n.SendTest(context.Background(), store.Channel{Name: "nuevo", Type: "ntfy"}, `{"topic":"t"}`)
	m := h.sent.last(t, "nuevo")
	if entry.Result != "ok" || entry.Event != "test" || entry.Attempts != 1 || m.Event != "test" || m.Language != "en" {
		t.Fatalf("entrada = %+v, mensaje = %+v", entry, m)
	}
	h.sent.fail = func(string, int) error { return &integrations.Error{Code: CodeRejected, Err: errors.New("400")} }
	entry = h.n.SendTest(context.Background(), store.Channel{Name: "nuevo", Type: "ntfy"}, `{"topic":"t"}`)
	if entry.Result != "error" || entry.Code != CodeRejected || entry.Attempts != 1 {
		t.Fatalf("entrada = %+v", entry)
	}
	if log, _ := h.st.ListLog(context.Background()); len(log) != 2 {
		t.Fatalf("registro = %+v", log)
	}
}

// Dos problemas a la vez se ven con segundos de diferencia, según el intervalo de
// cada servicio. Van en el mismo aviso si el segundo está a punto de madurar.
func TestGroupsNearlySimultaneous(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	a := h.service(t, "AdGuard", true)
	v := h.service(t, "VPN", true)
	h.set(a, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.set(v, "VPN", warn, "wgdashboard.down")
	h.step(t, 25*time.Second)
	h.step(t, 95*time.Second)
	m := h.sent.last(t, "móvil")
	if h.sent.count("móvil") != 1 || len(m.Changes) != 2 {
		t.Fatalf("mensajes = %d, cambios = %+v", h.sent.count("móvil"), m.Changes)
	}
	h.step(t, time.Hour)
	if h.sent.count("móvil") != 1 {
		t.Fatal("el adelantado se vuelve a avisar")
	}
}

// Uno que acaba de empezar no se adelanta: espera su propio plazo.
func TestGroupWindowIsLimited(t *testing.T) {
	h := newHarness(t)
	h.channel(t, "móvil")
	a := h.service(t, "AdGuard", true)
	v := h.service(t, "VPN", true)
	h.set(a, "AdGuard", warn, "adguard.protectionOff")
	h.step(t, 0)
	h.set(v, "VPN", warn, "wgdashboard.down")
	h.step(t, 80*time.Second)
	h.step(t, 40*time.Second)
	if m := h.sent.last(t, "móvil"); len(m.Changes) != 1 || m.Changes[0].Name != "AdGuard" {
		t.Fatalf("cambios = %+v", m.Changes)
	}
	h.step(t, 80*time.Second)
	if m := h.sent.last(t, "móvil"); h.sent.count("móvil") != 2 || m.Changes[0].Name != "VPN" {
		t.Fatalf("cambios = %+v", m.Changes)
	}
}

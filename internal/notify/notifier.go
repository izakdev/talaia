package notify

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/izakdev/talaia/internal/collector"
	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/store"
)

const (
	// tick es cada cuánto se revisan los cambios pendientes aunque no llegue ninguna lectura.
	tick = 5 * time.Second
	// groupWindow es lo máximo que se adelanta un cambio para ir en el aviso de otro.
	groupWindow = 30 * time.Second
	// CodeSecretUnreadable indica que el secreto del canal no se puede descifrar.
	CodeSecretUnreadable = "secret_unreadable"
)

// retries son las esperas antes de cada intento de envío.
var retries = []time.Duration{0, 30 * time.Second, 2 * time.Minute}

// Source es el recolector visto desde el notificador.
type Source interface {
	Snapshots() []collector.Snapshot
	Subscribe() (<-chan struct{}, func(), bool)
}

type candidate struct {
	tone  integrations.Tone
	since time.Time
}

// Notifier avisa por los canales activos cuando un servicio del panel cambia de
// color y el cambio dura el retardo configurado.
type Notifier struct {
	store *store.Store
	src   Source
	log   *slog.Logger

	now       func() time.Time
	newSender func(store.Channel, string) (Sender, error)
	sleep     func(context.Context, time.Duration) error

	mu         sync.Mutex
	candidates map[string]candidate
	wg         sync.WaitGroup
}

// New crea el notificador.
func New(st *store.Store, src Source, log *slog.Logger) *Notifier {
	return &Notifier{
		store: st, src: src, log: log,
		now: time.Now, newSender: NewSender, sleep: sleep,
		candidates: map[string]candidate{},
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Start lanza Run en segundo plano. Wait espera también a que termine.
func (n *Notifier) Start(ctx context.Context) {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		n.Run(ctx)
	}()
}

// Run revisa los servicios con cada lectura del recolector hasta que se cancele ctx.
func (n *Notifier) Run(ctx context.Context) {
	changes, unsubscribe, ok := n.src.Subscribe()
	if !ok {
		n.log.Error("el notificador no se puede suscribir al recolector")
		return
	}
	defer unsubscribe()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		if err := n.evaluate(ctx, n.now()); err != nil && ctx.Err() == nil {
			n.log.Error("no se pueden revisar los avisos", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-changes:
		case <-ticker.C:
		}
	}
}

// Wait espera a que terminen Run y los envíos en curso.
func (n *Notifier) Wait() { n.wg.Wait() }

// evaluate decide qué cambios han durado el retardo y los envía.
func (n *Notifier) evaluate(ctx context.Context, now time.Time) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	settings, err := n.store.GetSettings(ctx)
	if err != nil {
		return err
	}
	pages, err := n.store.Panel(ctx)
	if err != nil {
		return err
	}
	connections, err := n.store.ListConnections(ctx)
	if err != nil {
		return err
	}
	notified, err := n.store.NotifiedTones(ctx)
	if err != nil {
		return err
	}
	channels, err := n.store.ListChannels(ctx)
	if err != nil {
		return err
	}
	placed := map[string]bool{}
	for _, page := range pages {
		for _, w := range page.Widgets {
			placed[w.ConnectionID] = true
		}
	}
	silenced := map[string]bool{}
	for _, c := range connections {
		silenced[c.ID] = !c.Notify
	}
	var active []store.Channel
	for _, ch := range channels {
		if ch.Enabled {
			active = append(active, ch)
		}
	}

	// Un servicio fuera del panel no se vigila: se olvida su color.
	for id := range notified {
		if !placed[id] {
			if err := n.store.DeleteNotifiedTone(ctx, id); err != nil {
				return err
			}
		}
	}
	for id := range n.candidates {
		if !placed[id] {
			delete(n.candidates, id)
		}
	}

	delay := time.Duration(settings.NotifyDelaySeconds) * time.Second
	type pending struct {
		snap     collector.Snapshot
		previous integrations.Tone
		age      time.Duration
	}
	var waiting []pending
	for _, snap := range n.src.Snapshots() {
		if !placed[snap.ID] {
			continue
		}
		tone := snap.State.Tone
		// Un gris no dice nada del servicio: ni avisa ni reinicia el plazo.
		if tone != integrations.ToneGood && tone != integrations.ToneWarn && tone != integrations.ToneBad {
			continue
		}
		previous := integrations.Tone(notified[snap.ID])
		if previous == "" {
			previous = integrations.ToneGood
		}
		if tone == previous {
			delete(n.candidates, snap.ID)
			continue
		}
		c, ok := n.candidates[snap.ID]
		if !ok || c.tone != tone {
			c = candidate{tone: tone, since: now}
			n.candidates[snap.ID] = c
		}
		waiting = append(waiting, pending{snap, previous, now.Sub(c.since)})
	}
	// Cada servicio se consulta a su ritmo, así que dos problemas simultáneos se ven
	// con segundos de diferencia. Cuando uno madura, se adelantan los que están a
	// punto de hacerlo, para que vayan en el mismo aviso.
	threshold := delay
	if slices.ContainsFunc(waiting, func(p pending) bool { return p.age >= delay }) {
		threshold = delay - min(delay/2, groupWindow)
	}
	var changes []Change
	for _, p := range waiting {
		if p.age < threshold {
			continue
		}
		tone := p.snap.State.Tone
		// El color avisado se guarda al decidir: un canal caído no provoca repeticiones.
		if err := n.store.SetNotifiedTone(ctx, p.snap.ID, string(tone)); err != nil {
			return err
		}
		delete(n.candidates, p.snap.ID)
		if !silenced[p.snap.ID] && len(active) > 0 {
			changes = append(changes, Change{Name: p.snap.Name, Type: p.snap.Type, Tone: tone, Previous: p.previous, State: p.snap.State})
		}
	}
	if len(changes) == 0 {
		return nil
	}
	m := Compose(settings.NotifyLanguage, changes, now)
	for _, ch := range active {
		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			n.deliver(ctx, ch, m)
		}()
	}
	return nil
}

func logServices(m Message) []store.LogService {
	out := make([]store.LogService, 0, len(m.Changes))
	for _, c := range m.Changes {
		out = append(out, store.LogService{Name: c.Name, Tone: string(c.Tone), Previous: string(c.Previous)})
	}
	return out
}

// deliver envía un aviso a un canal con sus reintentos y lo anota en el registro.
func (n *Notifier) deliver(ctx context.Context, ch store.Channel, m Message) {
	entry := store.LogEntry{At: m.SentAt, Channel: ch.Name, ChannelType: ch.Type, Event: m.Event, Services: logServices(m)}
	err := n.attempt(ctx, ch, m, &entry, retries)
	n.record(ctx, entry, err)
}

// attempt hace los intentos de un envío. Antes de cada reintento comprueba que el
// canal sigue existiendo y activo.
func (n *Notifier) attempt(ctx context.Context, ch store.Channel, m Message, entry *store.LogEntry, delays []time.Duration) error {
	secret := ""
	if ch.ID != "" {
		var err error
		if secret, err = n.store.ChannelSecret(ctx, ch.ID); err != nil {
			return &integrations.Error{Code: CodeSecretUnreadable, Err: err}
		}
	}
	return n.attemptWith(ctx, ch, secret, m, entry, delays)
}

func (n *Notifier) attemptWith(ctx context.Context, ch store.Channel, secret string, m Message, entry *store.LogEntry, delays []time.Duration) error {
	sender, err := n.newSender(ch, secret)
	if err != nil {
		return &integrations.Error{Code: CodeSecretUnreadable, Err: err}
	}
	for i, d := range delays {
		if i > 0 {
			if n.sleep(ctx, d) != nil {
				return err
			}
			current, getErr := n.store.GetChannel(ctx, ch.ID)
			if getErr != nil || !current.Enabled {
				return err
			}
		}
		entry.Attempts++
		if err = sender.Send(ctx, m); err == nil || !Retryable(err) {
			return err
		}
	}
	return err
}

func (n *Notifier) record(ctx context.Context, entry store.LogEntry, err error) store.LogEntry {
	entry.Result = "ok"
	if err != nil {
		entry.Result, entry.Code = "error", integrations.Code(err)
		n.log.Warn("aviso no enviado", "canal", entry.Channel, "codigo", entry.Code, "intentos", entry.Attempts)
	} else {
		n.log.Info("aviso enviado", "canal", entry.Channel, "intentos", entry.Attempts)
	}
	// El registro se escribe aunque Talaia se esté cerrando.
	if logErr := n.store.AddLog(context.WithoutCancel(ctx), entry); logErr != nil {
		n.log.Error("no se puede guardar el registro de avisos", "error", logErr)
	}
	return entry
}

// SendTest envía un mensaje de prueba por un canal, guardado o no, en un solo
// intento, y lo anota en el registro.
func (n *Notifier) SendTest(ctx context.Context, ch store.Channel, secret string) store.LogEntry {
	lang := "es"
	if settings, err := n.store.GetSettings(ctx); err == nil && settings.NotifyLanguage != "" {
		lang = settings.NotifyLanguage
	}
	m := ComposeTest(lang, n.now())
	entry := store.LogEntry{At: m.SentAt, Channel: ch.Name, ChannelType: ch.Type, Event: m.Event, Services: logServices(m)}
	err := n.attemptWith(ctx, ch, secret, m, &entry, retries[:1])
	return n.record(ctx, entry, err)
}

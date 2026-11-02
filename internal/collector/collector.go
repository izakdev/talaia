// Package collector consulta los servicios en segundo plano y guarda el último estado.
package collector

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/internal/store"
)

// Estados de una conexión.
const (
	StatusPending = "pending"
	StatusOK      = "ok"
	StatusError   = "error"
)

const (
	collectTimeout = 30 * time.Second
	maxSubscribers = 256
	// CodeSecretUnreadable indica que la credencial no se puede descifrar.
	CodeSecretUnreadable = "secret_unreadable"
)

// Snapshot es el estado público de una conexión: sin URL, usuario ni credenciales.
type Snapshot struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Error         string     `json:"error,omitempty"`
	Data          any        `json:"data"`
	CheckedAt     *time.Time `json:"checked_at"`
	LastSuccessAt *time.Time `json:"last_success_at"`
}

type item struct {
	generation int
	cancel     context.CancelFunc
	target     string
	snapshot   Snapshot
}

// Manager mantiene un recolector por conexión.
type Manager struct {
	store    *store.Store
	registry integrations.Registry
	log      *slog.Logger

	mu         sync.Mutex
	base       context.Context
	wg         sync.WaitGroup
	generation int
	items      map[string]*item
	subs       map[chan struct{}]struct{}
}

// New crea el gestor.
func New(st *store.Store, registry integrations.Registry, log *slog.Logger) *Manager {
	return &Manager{
		store: st, registry: registry, log: log,
		items: map[string]*item{}, subs: map[chan struct{}]struct{}{},
	}
}

// Start arranca los recolectores hasta que se cancele ctx.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	m.base = ctx
	m.mu.Unlock()
	conns, err := m.store.ListConnections(ctx)
	if err != nil {
		return err
	}
	for _, c := range conns {
		m.start(c)
	}
	return nil
}

// Wait espera a que terminen los recolectores.
func (m *Manager) Wait() { m.wg.Wait() }

// Refresh aplica la configuración actual de una conexión y la consulta de inmediato.
func (m *Manager) Refresh(ctx context.Context, id string) error {
	c, err := m.store.GetConnection(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		m.Remove(id)
		return nil
	}
	if err != nil {
		return err
	}
	m.start(c)
	return nil
}

// Remove detiene una conexión y olvida su estado.
func (m *Manager) Remove(id string) {
	m.mu.Lock()
	if it, ok := m.items[id]; ok {
		it.cancel()
		delete(m.items, id)
		m.notifyLocked()
	}
	m.mu.Unlock()
}

func (m *Manager) start(c store.Connection) {
	factory, supported := m.registry[c.Type]
	if !supported {
		m.Remove(c.ID)
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.base == nil || m.base.Err() != nil {
		return
	}

	// Los datos de otro destino no pertenecen a esta configuración.
	target := strings.Join([]string{c.BaseURL, c.Username}, "\x00")
	snapshot := Snapshot{ID: c.ID, Name: c.Name, Type: c.Type, Status: StatusPending}
	if old, ok := m.items[c.ID]; ok {
		old.cancel()
		if old.target == target {
			snapshot = old.snapshot
			snapshot.Name = c.Name
		}
	}
	ctx, cancel := context.WithCancel(m.base)
	m.generation++
	generation := m.generation
	m.items[c.ID] = &item{generation: generation, cancel: cancel, target: target, snapshot: snapshot}
	m.notifyLocked()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer cancel()
		m.run(ctx, c, factory, generation)
	}()
}

func (m *Manager) run(ctx context.Context, c store.Connection, factory func(integrations.Config) integrations.Integration, generation int) {
	secret, err := m.store.ConnectionSecret(ctx, c.ID)
	if err != nil {
		if ctx.Err() == nil {
			m.log.Error("no se puede leer la credencial", "conexion", c.ID, "error", err)
			m.record(c.ID, generation, nil, CodeSecretUnreadable)
		}
		return
	}
	integration := factory(integrations.Config{
		BaseURL: c.BaseURL, Username: c.Username, Secret: secret, TLSSkipVerify: c.TLSSkipVerify,
	})
	ticker := time.NewTicker(time.Duration(c.IntervalSeconds) * time.Second)
	defer ticker.Stop()
	for {
		collectCtx, cancel := context.WithTimeout(ctx, collectTimeout)
		data, err := integration.Collect(collectCtx)
		cancel()
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			code := integrations.Code(err)
			if m.record(c.ID, generation, nil, code) {
				m.log.Warn("conexión con fallos", "conexion", c.Name, "codigo", code, "error", err)
			}
		} else if m.record(c.ID, generation, data, "") {
			m.log.Info("conexión correcta", "conexion", c.Name)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// record guarda un resultado y devuelve si ha cambiado el estado.
func (m *Manager) record(id string, generation int, data any, code string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.items[id]
	if !ok || it.generation != generation {
		return false
	}
	now := time.Now().UTC()
	s := &it.snapshot
	before := s.Status + s.Error
	s.CheckedAt = &now
	if code != "" {
		s.Status, s.Error = StatusError, code
	} else {
		s.Status, s.Error, s.Data, s.LastSuccessAt = StatusOK, "", data, &now
	}
	m.notifyLocked()
	return before != s.Status+s.Error
}

// Snapshots devuelve el último estado de cada conexión, ordenado por nombre.
func (m *Manager) Snapshots() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Snapshot, 0, len(m.items))
	for _, it := range m.items {
		out = append(out, it.snapshot)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Subscribe devuelve un canal que avisa de cambios. Los avisos seguidos se agrupan.
func (m *Manager) Subscribe() (<-chan struct{}, func(), bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.subs) >= maxSubscribers {
		return nil, nil, false
	}
	ch := make(chan struct{}, 1)
	m.subs[ch] = struct{}{}
	return ch, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		delete(m.subs, ch)
	}, true
}

func (m *Manager) notifyLocked() {
	for ch := range m.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

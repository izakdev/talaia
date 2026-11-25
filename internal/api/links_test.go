package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/izakdev/talaia/internal/store"
)

func link(id, name, url, icon string) map[string]any {
	return map[string]any{"id": id, "name": name, "url": url, "icon": icon}
}

func group(id, name string, links ...map[string]any) map[string]any {
	if links == nil {
		links = []map[string]any{}
	}
	return map[string]any{"id": id, "name": name, "links": links}
}

func TestSaveLinksValidation(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	many := func(groups, links int) []map[string]any {
		out := []map[string]any{}
		for i := range groups {
			g := group(fmt.Sprintf("g%d", i), "G")
			if i == 0 {
				ls := []map[string]any{}
				for j := range links {
					ls = append(ls, link(fmt.Sprintf("l%d", j), "L", "https://example.com/", ""))
				}
				g["links"] = ls
			}
			out = append(out, g)
		}
		return out
	}
	cases := []struct {
		name   string
		groups []map[string]any
		code   string
	}{
		{"id repetido", []map[string]any{group("a", "A", link("x", "X", "https://example.com/", "")), group("b", "B", link("x", "Y", "https://example.com/", ""))}, "invalid_links"},
		{"id de grupo inválido", []map[string]any{group("a b", "A")}, "invalid_links"},
		{"21 grupos", many(21, 0), "invalid_links"},
		{"101 enlaces", many(1, 101), "invalid_links"},
		{"grupo sin nombre", []map[string]any{group("a", "  ")}, "invalid_group_name"},
		{"grupo largo", []map[string]any{group("a", strings.Repeat("g", 41))}, "invalid_group_name"},
		{"enlace largo", []map[string]any{group("a", "A", link("x", strings.Repeat("n", 61), "https://example.com/", ""))}, "invalid_link_name"},
		{"enlace sin nombre", []map[string]any{group("a", "A", link("x", " ", "https://example.com/", ""))}, "invalid_link_name"},
		{"esquema ftp", []map[string]any{group("a", "A", link("x", "X", "ftp://example.com", ""))}, "invalid_link"},
		{"con usuario", []map[string]any{group("a", "A", link("x", "X", "https://user@example.com", ""))}, "invalid_link"},
		{"sin host", []map[string]any{group("a", "A", link("x", "X", "https://", ""))}, "invalid_link"},
		{"icono inexistente", []map[string]any{group("a", "A", link("x", "X", "https://example.com/", "no-existe"))}, "invalid_icon"},
	}
	for _, tc := range cases {
		if got := c.want(400, "PUT", "/api/admin/links", map[string]any{"groups": tc.groups}); !strings.Contains(got, tc.code) {
			t.Errorf("%s: %s, quería %s", tc.name, got, tc.code)
		}
	}
	if got := c.want(200, "GET", "/api/admin/links", nil); got != `{"groups":[]}`+"\n" {
		t.Fatalf("un error no debe guardar nada: %s", got)
	}
	ok := []map[string]any{group("red", "  Red ", link("router", " Router ", " http://192.0.2.1/ ", "pihole")), group("vacio", "Vacío")}
	body := c.want(200, "PUT", "/api/admin/links", map[string]any{"groups": ok})
	want := `{"groups":[{"id":"red","name":"Red","links":[{"id":"router","name":"Router","url":"http://192.0.2.1/","icon":"pihole"}]},{"id":"vacio","name":"Vacío","links":[]}]}`
	if strings.TrimSpace(body) != want {
		t.Fatalf("guardado = %s", body)
	}
	if got := c.want(200, "GET", "/api/admin/links", nil); strings.TrimSpace(got) != want {
		t.Fatalf("leído = %s", got)
	}
}

func TestSaveLinksKeepsRetiredIcon(t *testing.T) {
	c, st := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	if err := st.SaveLinkGroups(context.Background(), []store.LinkGroup{{ID: "a", Name: "A", Links: []store.Link{{ID: "x", Name: "X", URL: "https://example.com/", Icon: "retirado"}}}}); err != nil {
		t.Fatal(err)
	}
	c.want(200, "PUT", "/api/admin/links", map[string]any{"groups": []map[string]any{group("a", "A", link("x", "X", "https://example.com/", "retirado"))}})
	// Otro enlace no puede elegir un icono que no está en el índice.
	c.want(400, "PUT", "/api/admin/links", map[string]any{"groups": []map[string]any{group("a", "A", link("y", "Y", "https://example.com/", "retirado"))}})
	// En el panel, el icono retirado se pinta como sin icono.
	waitDashboard(t, newClient(t, c.base), `"links":[{"name":"X","url":"https://example.com/","icon":null}]`)
}

func TestLinksRequireAuth(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	c.want(204, "POST", "/api/auth/logout", nil)
	c.want(401, "GET", "/api/admin/links", nil)
	c.want(401, "PUT", "/api/admin/links", map[string]any{"groups": []map[string]any{}})
}

func TestDashboardLinkGroups(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	c.want(201, "POST", "/api/admin/connections", adguardConn("correcta"))
	c.want(200, "PUT", "/api/admin/links", map[string]any{"groups": []map[string]any{
		group("vacio", "Vacío"),
		group("red", "Red", link("router", "Router", "http://192.0.2.1/", "pihole"), link("switch", "Switch", "http://192.0.2.2/", "")),
	}})
	public := newClient(t, c.base)
	body := waitDashboard(t, public, `"link_groups"`)
	var state struct {
		LinkGroups  json.RawMessage `json:"link_groups"`
		Connections []map[string]any
	}
	if err := json.Unmarshal([]byte(body), &state); err != nil {
		t.Fatal(err)
	}
	want := `[{"name":"Red","links":[{"name":"Router","url":"http://192.0.2.1/","icon":{"slug":"pihole","title":"Pi-hole","hex":"96060C"}},{"name":"Switch","url":"http://192.0.2.2/","icon":null}]}]`
	if string(state.LinkGroups) != want {
		t.Fatalf("link_groups = %s", state.LinkGroups)
	}
	if len(state.Connections) != 1 || state.Connections[0]["show_access"] != true {
		t.Fatalf("conexiones = %v", state.Connections)
	}
}

func TestConnectionShowAccess(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	created := c.want(201, "POST", "/api/admin/connections", adguardConn("correcta"))
	if !strings.Contains(created, `"show_access":true`) {
		t.Fatalf("sin el campo debe valer true: %s", created)
	}
	var out connectionOut
	if err := json.Unmarshal([]byte(created), &out); err != nil {
		t.Fatal(err)
	}
	m := adguardConn("")
	m["show_access"] = false
	if got := c.want(200, "PUT", "/api/admin/connections/"+out.ID, m); !strings.Contains(got, `"show_access":false`) {
		t.Fatalf("debe guardar false: %s", got)
	}
	waitDashboard(t, newClient(t, c.base), `"show_access":false`)
}

func TestSaveLinksAtLimits(t *testing.T) {
	c, _ := newServer(t)
	c.want(200, "POST", "/api/setup", creds())
	// 100 enlaces con la URL más larga admitida caben en una sola petición.
	long := "https://example.com/" + strings.Repeat("a", maxLinkURLLen-len("https://example.com/"))
	links := []map[string]any{}
	for i := range maxLinks {
		links = append(links, link(fmt.Sprintf("l%d", i), strings.Repeat("n", maxLinkNameLen), long, ""))
	}
	c.want(200, "PUT", "/api/admin/links", map[string]any{"groups": []map[string]any{group("a", "A", links...)}})
}

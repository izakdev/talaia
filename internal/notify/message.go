// Package notify avisa por Telegram, ntfy o webhook de los cambios de estado de los servicios.
package notify

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
	"github.com/izakdev/talaia/web"
)

// Eventos de un mensaje.
const (
	EventChange = "change"
	EventTest   = "test"
)

// maxName es la longitud máxima de un nombre de servicio en un aviso.
const maxName = 80

var texts = map[string]map[string]string{
	"es": {
		"attention": "Talaia · Necesita atención",
		"recovered": "Talaia · Vuelve a estar en orden",
		"testTitle": "Talaia · Mensaje de prueba",
		"testBody":  "Si lo ves, este canal funciona.",
	},
	"en": {
		"attention": "Talaia · Needs attention",
		"recovered": "Talaia · Back to normal",
		"testTitle": "Talaia · Test message",
		"testBody":  "If you can see it, this channel works.",
	},
}

var emoji = map[integrations.Tone]string{
	integrations.ToneBad:  "🔴",
	integrations.ToneWarn: "🟠",
	integrations.ToneGood: "🟢",
}

// Change es el cambio de color de un servicio.
type Change struct {
	Name     string
	Type     string
	Tone     integrations.Tone
	Previous integrations.Tone
	State    integrations.State
}

// Message es un aviso listo para cualquier canal. No lleva direcciones ni secretos.
type Message struct {
	Event    string
	Language string
	Title    string
	Text     string
	Worst    integrations.Tone
	Changes  []Change
	SentAt   time.Time
}

func language(lang string) string {
	if _, ok := texts[lang]; ok {
		return lang
	}
	return "es"
}

// Label traduce el estado de un servicio, igual que el panel.
func Label(lang string, s integrations.State) string {
	text, ok := web.StateLabels()[language(lang)][s.Key]
	if !ok {
		return s.Key
	}
	for k, v := range s.Params {
		text = strings.ReplaceAll(text, "{"+k+"}", fmt.Sprint(v))
	}
	return text
}

// cleanName evita que un nombre rompa el formato del mensaje.
func cleanName(name string) string {
	name = strings.Join(strings.Fields(name), " ")
	if r := []rune(name); len(r) > maxName {
		name = string(r[:maxName])
	}
	return name
}

// Compose crea el aviso de unos cambios: primero los problemas, del más grave al
// menos grave, y después los servicios que vuelven a estar en orden.
func Compose(lang string, changes []Change, now time.Time) Message {
	lang = language(lang)
	sorted := make([]Change, len(changes))
	for i, c := range changes {
		c.Name = cleanName(c.Name)
		sorted[i] = c
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		return integrations.Severity(sorted[i].Tone) > integrations.Severity(sorted[j].Tone)
	})
	var problems, recovered []string
	worst := integrations.ToneGood
	for _, c := range sorted {
		line := c.Name + ": " + Label(lang, c.State)
		if integrations.Severity(c.Tone) > 0 {
			problems = append(problems, line)
		} else {
			recovered = append(recovered, line)
		}
		if integrations.Severity(c.Tone) > integrations.Severity(worst) {
			worst = c.Tone
		}
	}
	var blocks []string
	if len(problems) > 0 {
		blocks = append(blocks, emoji[worst]+" "+texts[lang]["attention"]+"\n"+strings.Join(problems, "\n"))
	}
	if len(recovered) > 0 {
		blocks = append(blocks, emoji[integrations.ToneGood]+" "+texts[lang]["recovered"]+"\n"+strings.Join(recovered, "\n"))
	}
	text := strings.Join(blocks, "\n\n")
	title, _, _ := strings.Cut(text, "\n")
	return Message{Event: EventChange, Language: lang, Title: title, Text: text, Worst: worst, Changes: sorted, SentAt: now}
}

// ComposeTest crea el mensaje del botón «Enviar prueba».
func ComposeTest(lang string, now time.Time) Message {
	lang = language(lang)
	title := "🔔 " + texts[lang]["testTitle"]
	text := title + ". " + texts[lang]["testBody"]
	return Message{Event: EventTest, Language: lang, Title: title, Text: text, Worst: integrations.ToneGood, Changes: []Change{}, SentAt: now}
}

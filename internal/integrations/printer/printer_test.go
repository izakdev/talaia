package printer

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/izakdev/talaia/internal/integrations"
)

// attr es un atributo de la respuesta simulada: etiqueta, nombre y valores.
type attr struct {
	tag    byte
	name   string
	values []any
}

func encodeValue(b *bytes.Buffer, tag byte, name string, v any) {
	b.WriteByte(tag)
	binary.Write(b, binary.BigEndian, uint16(len(name)))
	b.WriteString(name)
	var raw []byte
	switch x := v.(type) {
	case int:
		raw = binary.BigEndian.AppendUint32(nil, uint32(int32(x)))
	case bool:
		raw = []byte{0}
		if x {
			raw[0] = 1
		}
	case string:
		raw = []byte(x)
	case []byte:
		raw = x
	}
	binary.Write(b, binary.BigEndian, uint16(len(raw)))
	b.Write(raw)
}

// encode arma una respuesta IPP 2.0 con el status-code y los atributos dados.
func encode(status uint16, attrs []attr) []byte {
	var b bytes.Buffer
	b.Write([]byte{2, 0})
	binary.Write(&b, binary.BigEndian, status)
	b.Write([]byte{0, 0, 0, 1, tagOperation})
	encodeValue(&b, tagCharset, "attributes-charset", "utf-8")
	encodeValue(&b, tagLanguage, "attributes-natural-language", "en")
	b.WriteByte(0x04) // printer-attributes-tag
	for _, a := range attrs {
		if a.tag == tagBegColl {
			encodeValue(&b, tagBegColl, a.name, "")
			encodeValue(&b, 0x4A, "", "media-size")
			encodeValue(&b, tagKeyword, "", "secreto-en-coleccion")
			encodeValue(&b, tagEndColl, "", "")
			continue
		}
		for i, v := range a.values {
			name := a.name
			if i > 0 {
				name = ""
			}
			encodeValue(&b, a.tag, name, v)
		}
	}
	b.WriteByte(tagEnd)
	return b.Bytes()
}

// textLang codifica un textWithLanguage.
func textLang(lang, text string) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(lang)))
	b = append(b, lang...)
	b = binary.BigEndian.AppendUint16(b, uint16(len(text)))
	return append(b, text...)
}

// realShape copia la forma de una Brother DCP-L3550CDW real (firmware de 2022),
// con nombre, ubicación y direcciones inventados.
func realShape(state int, reasons []string, levels []int) []attr {
	rs := make([]any, len(reasons))
	for i, r := range reasons {
		rs[i] = r
	}
	ls := make([]any, len(levels))
	for i, l := range levels {
		ls[i] = l
	}
	return []attr{
		{tagURI, "printer-uri-supported", []any{"ipp://192.0.2.40:631/ipp/print"}},
		{0x42, "printer-name", []any{"impresora-simulada"}},
		{0x41, "printer-location", []any{"Despacho simulado"}},
		{tagTextLang, "printer-info", []any{textLang("es", "Impresora del despacho simulado")}},
		{tagURI, "printer-uuid", []any{"urn:uuid:00000000-0000-4000-8000-000000000001"}},
		{0x41, "printer-device-id", []any{"MFG:Brother;CMD:PJL;SN:SIM-SERIE-0001;"}},
		{0x42, "printer-dns-sd-name", []any{"Brother simulada.simulado"}},
		{0x41, "printer-make-and-model", []any{"Brother DCP-L3550CDW series"}},
		{tagEnum, "printer-state", []any{state}},
		{tagKeyword, "printer-state-reasons", rs},
		{tagBoolean, "printer-is-accepting-jobs", []any{true}},
		{tagTextLang, "printer-alert-description", []any{textLang("es", "Espera")}},
		{tagBegColl, "media-col-ready", nil},
		{0x42, "marker-colors", []any{"#000000", "#00FFFF", "#FF00FF", "#FFFF00"}},
		{tagInteger, "marker-high-levels", []any{100, 100, 100, 100}},
		{tagInteger, "marker-levels", ls},
		{tagInteger, "marker-low-levels", []any{10, 10, 10, 10}},
		{0x42, "marker-names", []any{"BK", "C", "M", "Y"}},
		{tagKeyword, "marker-types", []any{"toner", "toner", "toner", "toner"}},
		{0x42, "printer-firmware-name", []any{"MAIN", "SUB1"}},
		{0x41, "printer-firmware-string-version", []any{"ZA2206202233:93D6", "1.59"}},
	}
}

type fakePrinter struct {
	srv  *httptest.Server
	last []byte
}

func serve(t *testing.T, handler func(w http.ResponseWriter, r *http.Request)) *fakePrinter {
	t.Helper()
	f := &fakePrinter{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.last, _ = io.ReadAll(r.Body)
		handler(w, r)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func ippReply(body []byte) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != DefaultPath || r.Header.Get("Content-Type") != "application/ipp" {
			http.Error(w, "no", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/ipp")
		w.Write(body)
	}
}

func collect(t *testing.T, f *fakePrinter) (any, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	base := strings.Replace(f.srv.URL, "http://", "ipp://", 1)
	return New(integrations.Config{BaseURL: base}).Collect(ctx)
}

func TestTarget(t *testing.T) {
	ok := []struct{ in, endpoint, uri string }{
		{"ipp://impresora.simulado", "http://impresora.simulado:631/ipp/print", "ipp://impresora.simulado:631/ipp/print"},
		{"ipp://impresora.simulado/", "http://impresora.simulado:631/ipp/print", "ipp://impresora.simulado:631/ipp/print"},
		{"ipps://192.0.2.40:443/ipp/print", "https://192.0.2.40:443/ipp/print", "ipps://192.0.2.40:443/ipp/print"},
		{"ipp://192.0.2.40:8631/printers/laser", "http://192.0.2.40:8631/printers/laser", "ipp://192.0.2.40:8631/printers/laser"},
		{"ipp://[2001:db8::40]", "http://[2001:db8::40]:631/ipp/print", "ipp://[2001:db8::40]:631/ipp/print"},
	}
	for _, c := range ok {
		endpoint, uri, err := Target(c.in)
		if err != nil || endpoint != c.endpoint || uri != c.uri {
			t.Errorf("Target(%q) = %q, %q, %v", c.in, endpoint, uri, err)
		}
	}
	for _, bad := range []string{
		"http://impresora.simulado", "ipp://", "ipp://usuario@impresora.simulado", "ipp://impresora.simulado:0",
		"ipp://impresora.simulado:99999", "ipp://impresora.simulado?x=1", "ipp://impresora.simulado#x",
	} {
		if _, _, err := Target(bad); err == nil {
			t.Errorf("Target(%q) debería fallar", bad)
		}
	}
}

func TestCollectReadsRealShape(t *testing.T) {
	f := serve(t, ippReply(encode(0, realShape(3, []string{"none"}, []int{90, 85, 8, -3}))))
	got, err := collect(t, f)
	if err != nil {
		t.Fatal(err)
	}
	d := got.(Data)
	if d.Model != "Brother DCP-L3550CDW series" || d.Firmware != "ZA2206202233:93D6" || d.Status != StateIdle || !d.AcceptingJobs {
		t.Fatalf("datos = %+v", d)
	}
	if len(d.Reasons) != 0 || len(d.Markers) != 4 {
		t.Fatalf("motivos o consumibles = %+v", d)
	}
	black, yellow := d.Markers[0], d.Markers[3]
	if black.Name != "BK" || black.Color != "#000000" || black.Type != "toner" || *black.Level != 90 || black.Low != 10 {
		t.Fatalf("negro = %+v", black)
	}
	if yellow.Level != nil {
		t.Fatalf("un nivel -3 es desconocido: %+v", yellow)
	}
	// La petición es Get-Printer-Attributes con la printer-uri de la dirección y sin credenciales.
	if f.last[2] != 0 || f.last[3] != 0x0B || !bytes.Contains(f.last, []byte("ipp://127.0.0.1:")) || !bytes.Contains(f.last, []byte("marker-levels")) {
		t.Fatalf("petición = %q", f.last)
	}
}

func TestReasons(t *testing.T) {
	f := serve(t, ippReply(encode(0, realShape(5, []string{"media-jam-error", "toner-low-warning", "cover-open", "sleep-report", "fuser-under-temp-warning", "brother-raro-warning", "media-jam-error"}, []int{90, 90, 90, 90}))))
	got, err := collect(t, f)
	if err != nil {
		t.Fatal(err)
	}
	want := []Reason{{"media-jam", SeverityError}, {"toner-low", SeverityWarning}, {"cover-open", SeverityError}, {"other", SeverityWarning}}
	b, _ := json.Marshal(got.(Data).Reasons)
	w, _ := json.Marshal(want)
	if string(b) != string(w) {
		t.Fatalf("motivos = %s, quiero %s", b, w)
	}
	if got.(Data).Status != StateStopped {
		t.Fatalf("estado = %q", got.(Data).Status)
	}
}

func TestPrinterDataHidesIdentity(t *testing.T) {
	f := serve(t, ippReply(encode(0, realShape(4, []string{"none"}, []int{50, 50, 50, 50}))))
	got, err := collect(t, f)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(got)
	for _, leak := range []string{"192.0.2", "127.0.0.1", "impresora-simulada", "Despacho", "uuid", "SIM-SERIE", "simulado", "secreto-en-coleccion", "Espera", "ipp://"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("los datos publican %q: %s", leak, b)
		}
	}
}

func TestPrinterErrors(t *testing.T) {
	cases := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		code    string
	}{
		{"HTTP 401", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusUnauthorized) }, integrations.CodeUnauthorized},
		{"IPP no autorizado", ippReply(encode(0x0403, nil)), integrations.CodeUnauthorized},
		{"IPP error del servidor", ippReply(encode(0x0500, nil)), integrations.CodeBadResponse},
		{"HTTP 500", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) }, integrations.CodeBadResponse},
		{"página web", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte("<html>"))
		}, integrations.CodeBadResponse},
		{"cortada", ippReply(encode(0, realShape(3, nil, []int{1}))[:60]), integrations.CodeBadResponse},
		{"sin estado", ippReply(encode(0, []attr{{0x41, "printer-make-and-model", []any{"X"}}})), integrations.CodeBadResponse},
		{"estado raro", ippReply(encode(0, []attr{{tagEnum, "printer-state", []any{9}}})), integrations.CodeBadResponse},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := collect(t, serve(t, c.handler))
			if got := integrations.Code(err); got != c.code {
				t.Fatalf("código = %q (%v), quiero %q", got, err, c.code)
			}
		})
	}
	// Sin servicio, el fallo es de red.
	f := serve(t, ippReply(nil))
	f.srv.Close()
	if _, err := collect(t, f); integrations.Code(err) != integrations.CodeUnreachable {
		t.Fatalf("sin servicio: %v", err)
	}
}

func TestParseIgnoresBadMarkers(t *testing.T) {
	attrs := []attr{
		{tagEnum, "printer-state", []any{3}},
		{0x42, "marker-names", []any{"Negro", strings.Repeat("x", 300)}},
		{0x42, "marker-colors", []any{"javascript:alert(1)", "#00ff00"}},
		{tagKeyword, "marker-types", []any{"plutonio", "ink"}},
		{tagInteger, "marker-levels", []any{150, -2}},
		{tagInteger, "marker-low-levels", []any{-1, 20}},
	}
	got, err := collect(t, serve(t, ippReply(encode(0, attrs))))
	if err != nil {
		t.Fatal(err)
	}
	m := got.(Data).Markers
	if m[0].Color != "" || m[0].Type != "other" || m[0].Level != nil || m[0].Low != 10 {
		t.Fatalf("primero = %+v", m[0])
	}
	if len([]rune(m[1].Name)) != maxText || m[1].Color != "#00FF00" || m[1].Type != "ink" || m[1].Low != 20 {
		t.Fatalf("segundo = %+v", m[1])
	}
}

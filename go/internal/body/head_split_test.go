package body

import "testing"

func TestSplitFirstSentence(t *testing.T) {
	for _, c := range []struct{ in, first, rest string }{
		{"En Juárez hace 16 grados y está nublado. Mañana sube a 22 y no llueve.", "En Juárez hace 16 grados y está nublado.", "Mañana sube a 22 y no llueve."},
		{"¡Hola! Estoy bien, rodando y listo para ayudar. ¿Y tú?", "¡Hola! Estoy bien, rodando y listo para ayudar.", ""},
		{"Listo, apagué la luz.", "Listo, apagué la luz.", ""},
		{"Encontré un Costco en Rancho Agua Caliente. Si quieres, lo busco otra vez con más detalle.", "Encontré un Costco en Rancho Agua Caliente.", "Si quieres, lo busco otra vez con más detalle."},
	} {
		f, r := splitFirstSentence(c.in)
		if c.rest == "" {
			if r != "" || f != c.in {
				t.Errorf("%q: got %q | %q, want no split", c.in, f, r)
			}
			continue
		}
		if f != c.first || r != c.rest {
			t.Errorf("%q: got %q | %q", c.in, f, r)
		}
	}
}

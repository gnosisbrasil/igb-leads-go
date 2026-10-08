package service

import "testing"

// TestTemplateKeyMapping guards the Meta→DB key map: every mapped key
// must exist in the seeds, otherwise sends silently skip.
func TestTemplateKeyMapping(t *testing.T) {
	keysOf := func(objective string) map[string]bool {
		out := map[string]bool{}
		for _, tpl := range DefaultTemplates[objective] {
			out[tpl.Key] = true
		}
		return out
	}
	main := keysOf("camara_publica")
	for meta, key := range keyFor {
		if meta == "gnosis_lembrete" {
			continue // covered with its fallback below
		}
		if !main[key] {
			t.Fatalf("keyFor[%q] = %q ausente nos seeds", meta, key)
		}
	}
	if !main[keyFor["gnosis_lembrete"]] {
		t.Fatalf("lembrete_evento ausente nos seeds")
	}
	primeira := keysOf("primeira_camara")
	if !primeira[keyFallback["gnosis_lembrete"]] {
		t.Fatal("fallback lembrete_proxima_aula ausente nos seeds primeira_camara")
	}
}

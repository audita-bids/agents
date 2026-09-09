package service

import (
	"slices"
	"testing"
)

func TestMatchedKeywords(t *testing.T) {
	cases := []struct {
		name     string
		keywords []string
		text     string
		want     []string
	}{
		{
			name:     "variação de grafia e sufixo",
			keywords: []string{"tecnologia"},
			text:     "Contratação de empresa de base tecnológica para suporte",
			want:     []string{"tecnologia"},
		},
		{
			name:     "plural e acento",
			keywords: []string{"poços artesianos"},
			text:     "execução de 14 (catorze) poços artesianos tubulares profundos",
			want:     []string{"poços artesianos"},
		},
		{
			name:     "singular alcança o plural",
			keywords: []string{"obra"},
			text:     "Contratação de obras de pavimentação asfáltica",
			want:     []string{"obra"},
		},
		{
			name:     "mesmo setor não é aderência",
			keywords: []string{"merenda escolar"},
			text:     "Aquisição de uniforme escolar para a rede municipal",
			want:     nil,
		},
		{
			name:     "termo ausente não casa",
			keywords: []string{"buffet"},
			text:     "execução de poços artesianos tubulares profundos",
			want:     nil,
		},
		{
			name:     "palavra curta sozinha não vira radical",
			keywords: []string{"de"},
			text:     "Contratação de serviços diversos",
			want:     nil,
		},
		{
			name:     "duas palavras-chave, uma casa",
			keywords: []string{"poços artesianos", "buffet"},
			text:     "regularização de 14 poços artesianos do município",
			want:     []string{"poços artesianos"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := matchedKeywords(c.keywords, c.text)

			if !slices.Equal(got, c.want) {
				t.Errorf("matchedKeywords(%q, %q) = %v, quer %v", c.keywords, c.text, got, c.want)
			}
		})
	}
}

package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type NoticeExtraction struct {
	Object           string   `json:"object"`
	Modality         string   `json:"modality"`
	ProcessNumber    string   `json:"process_number"`
	Agency           Agency   `json:"agency"`
	EstimatedValue   string   `json:"estimated_value"`
	OpeningDate      string   `json:"opening_date"`
	JudgmentCriteria string   `json:"judgment_criteria"`
	Qualifications   []string `json:"qualifications"`
	Flags            []string `json:"flags"`
	Summary          string   `json:"summary"`
	Score            int      `json:"score"`
	ScoreRationale   string   `json:"score_rationale"`
	MatchedKeywords  []string `json:"matched_keywords"`
}

type Agency struct {
	Name string `json:"name"`
	CNPJ string `json:"cnpj"`
	UASG string `json:"uasg"`
}

const systemPrompt = `Role: analista de editais públicos BR (Lei 14.133/2021) da Audita
Lang: pt-BR
Null: use string vazia "" quando o dado não constar — nunca inferir
flags: no máximo os 4 riscos/ambiguidades jurídicas mais relevantes
qualifications: no máximo as 6 exigências de habilitação mais críticas
estimated_value: apenas o número, sem R$, sem texto por extenso. Ex: 2324342.04
object: o serviço/obra/fornecimento que será contratado, exatamente como descrito, sem ocultar informações
summary: resumo executivo de 2-3 frases — o que será contratado, valor e prazo relevante
score: inteiro 0-100 medindo EXCLUSIVAMENTE a correspondência entre o objeto/escopo do edital e as palavras-chave do cliente. SEJA RIGOROSO: comece de 0 e pontue apenas correspondência real. 85-100: o objeto central É o que as palavras-chave descrevem; 60-84: parte relevante do escopo corresponde diretamente a alguma palavra-chave; 30-59: correspondência apenas parcial ou tangencial; 0-29: nenhuma correspondência. Mesmo setor NÃO é aderência: "merenda escolar" não adere a "uniforme escolar". Sem correspondência real, o score DEVE ser ≤ 25
matched_keywords: as palavras-chave do cliente que de fato correspondem ao objeto/escopo (sinônimos diretos contam); array vazio se nenhuma corresponder
score_rationale: 1-2 frases citando quais palavras-chave casaram com quais termos do edital — ou afirmando que nenhuma casou
Output: JSON puro, sem markdown`

type Client struct {
	client *openai.Client
}

func NewOpenaiClient() *Client {
	v, ok := os.LookupEnv("OPENAI_API_KEY")
	if !ok {
		panic("OPENAI_API_KEY not found")
	}
	c := openai.NewClient(option.WithAPIKey(v))
	return &Client{client: &c}
}

var ragQueries = map[string]string{
	"object":            "1. DO OBJETO cláusula primeira objeto contratação descrição serviço obra fornecimento creche escola hospital",
	"modality":          "modalidade pregão eletrônico concorrência tomada de preços convite leilão dispensa inexigibilidade",
	"judgment_criteria": "critério de julgamento menor preço melhor técnica técnica e preço",
	"preamble":          "edital processo administrativo número data da sessão pública abertura valor total estimado",
}

var ragNPerField = map[string]int{
	"object":   4,
	"preamble": 2,
}

const (
	maxDocChars = 120_000
	directChars = 12_000
)

var whitespaceRe = regexp.MustCompile(`[ \t]{2,}`)
var blankLinesRe = regexp.MustCompile(`\n{3,}`)

func compactText(s string) string {
	s = whitespaceRe.ReplaceAllString(s, " ")
	s = blankLinesRe.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

var sectionHeadingRe = regexp.MustCompile(
	`(?:^|\n)\s*(?:SE[ÇC][ÃA]O\s+[IVXL]+\s*[-–—]?\s*)?(?:\d{1,2}\s*[-–.)]\s*)?D[AO]S?\s+[A-ZÁÂÃÉÊÍÓÔÕÚÇ]{3,}[^\n]{0,80}`,
)
var relevantSectionRe = regexp.MustCompile(
	`OBJETO|HABILITA|JULGAMENTO|PROPOST|SESS[ÃA]O|ABERTURA|VALOR|PRE[ÇC]O|CRIT[ÉE]RIO|IMPUGNA`,
)

const (
	// Document head kept whole: process number, modality, dates, value.
	preambleChars = 6_000
	// Cap per kept section (habilitação can sprawl).
	maxSectionChars = 20_000
	// Below this the filter probably matched noise — keep the full text.
	minFilteredChars = 8_000
)

func filterSections(text string) string {
	locs := sectionHeadingRe.FindAllStringIndex(text, -1)
	if len(locs) < 4 {
		return text
	}

	var sb strings.Builder

	head := locs[0][0]
	if head > preambleChars {
		head = preambleChars
	}
	sb.WriteString(text[:head])

	for i, loc := range locs {
		if !relevantSectionRe.MatchString(text[loc[0]:loc[1]]) {
			continue
		}
		end := len(text)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		if end-loc[0] > maxSectionChars {
			end = loc[0] + maxSectionChars
		}
		sb.WriteString("\n\n")
		sb.WriteString(text[loc[0]:end])
	}

	if filtered := sb.String(); len(filtered) >= minFilteredChars {
		return filtered
	}
	return text
}

func (c *Client) AnalyzeNotice(ctx context.Context, r io.Reader, keywords []string) (*NoticeExtraction, string, error) {
	text, err := ExtractText(r)
	if err != nil {
		return nil, "", fmt.Errorf("extract pdf: %w", err)
	}

	text = compactText(text)

	if len(text) <= directChars {
		return c.extract(ctx, map[string][]string{"documento": {text}}, keywords)
	}

	// Large document: keep only the sections the extraction needs, cap the
	// rest, and re-check — the filtered text often fits the direct path.
	text = filterSections(text)
	if len(text) > maxDocChars {
		text = strings.ToValidUTF8(text[:maxDocChars], "")
	}
	if len(text) <= directChars {
		return c.extract(ctx, map[string][]string{"documento": {text}}, keywords)
	}

	rag, err := NewRAG()
	if err != nil {
		return nil, "", fmt.Errorf("rag init: %w", err)
	}

	if err := rag.LoadText(ctx, text); err != nil {
		return nil, "", fmt.Errorf("load text: %w", err)
	}

	fieldChunks, err := rag.FindRelevantBatch(ctx, ragQueries, ragNPerField, 3)
	if err != nil {
		return nil, "", fmt.Errorf("rag batch: %w", err)
	}

	return c.extract(ctx, fieldChunks, keywords)
}

func (c *Client) extract(ctx context.Context, fieldChunks map[string][]string, keywords []string) (*NoticeExtraction, string, error) {
	var sb strings.Builder

	sb.WriteString("### Perfil do cliente\n")
	if len(keywords) > 0 {
		sb.WriteString("Palavras-chave: " + strings.Join(keywords, ", ") + "\n\n")
	} else {
		sb.WriteString("Nenhuma palavra-chave configurada.\n\n")
	}

	sb.WriteString("Extraia os campos e calcule o score a partir dos trechos do edital abaixo.\n\n")

	// Stable field order (deterministic prompts) and chunk dedupe — the same
	// excerpt often ranks for more than one query and would be paid twice.
	fields := make([]string, 0, len(fieldChunks))
	for field := range fieldChunks {
		fields = append(fields, field)
	}
	sort.Strings(fields)

	seen := make(map[string]struct{})
	for _, field := range fields {
		unique := make([]string, 0, len(fieldChunks[field]))
		for _, ch := range fieldChunks[field] {
			if _, dup := seen[ch]; dup {
				continue
			}
			seen[ch] = struct{}{}
			unique = append(unique, ch)
		}
		if len(unique) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", field, buildContext(unique)))
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(sb.String()),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "notice_analysis",
					Strict: openai.Bool(true),
					Schema: ResumeNoticeSchema(),
				},
			},
		},
		MaxCompletionTokens: openai.Int(600),
		Temperature:         openai.Float(0),
	})
	if err != nil {
		return nil, "", fmt.Errorf("openai: %w", err)
	}

	raw := resp.Choices[0].Message.Content

	var result NoticeExtraction
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, "", fmt.Errorf("parse: %w", err)
	}
	return &result, raw, nil
}

func buildContext(chunks []string) string {
	var sb strings.Builder
	for i, ch := range chunks {
		sb.WriteString(fmt.Sprintf("\n--- Trecho %d ---\n%s\n", i+1, ch))
	}
	return sb.String()
}

func (c *Client) ResumeBase64(_ context.Context, b64 string) (io.Reader, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, errors.New("error trying to decode b64 document")
	}
	return bytes.NewReader(raw), nil
}

func ResumeNoticeSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"object", "modality", "process_number", "agency", "estimated_value",
			"opening_date", "judgment_criteria", "qualifications", "flags",
			"summary", "score", "score_rationale", "matched_keywords",
		},
		"properties": map[string]interface{}{
			"object":            map[string]interface{}{"type": "string", "description": "Objeto da licitação — o serviço, obra ou fornecimento a ser contratado. Nunca retorne instruções procedimentais."},
			"modality":          map[string]interface{}{"type": "string"},
			"process_number":    map[string]interface{}{"type": "string"},
			"estimated_value":   map[string]interface{}{"type": "string", "description": "Apenas o número. Ex: 2324342.04"},
			"opening_date":      map[string]interface{}{"type": "string"},
			"judgment_criteria": map[string]interface{}{"type": "string", "description": "Critério de julgamento: menor preço, melhor técnica, técnica e preço etc."},
			"agency": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"name", "cnpj", "uasg"},
				"properties": map[string]interface{}{
					"name": map[string]interface{}{"type": []string{"string", "null"}},
					"cnpj": map[string]interface{}{"type": []string{"string", "null"}},
					"uasg": map[string]interface{}{"type": []string{"string", "null"}},
				},
			},
			"qualifications": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "As exigências de habilitação mais críticas. No máximo 6.",
			},
			"flags": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Riscos/ambiguidades jurídicas relevantes. No máximo 4.",
			},
			"summary":         map[string]interface{}{"type": "string", "description": "Resumo executivo de 2-3 frases."},
			"score":           map[string]interface{}{"type": "integer", "description": "Correspondência 0-100 entre o objeto do edital e as palavras-chave do cliente. Rigoroso: sem correspondência real, ≤ 25."},
			"score_rationale": map[string]interface{}{"type": "string", "description": "Quais palavras-chave casaram com quais termos do edital — ou que nenhuma casou."},
			"matched_keywords": map[string]interface{}{
				"type":        "array",
				"items":       map[string]interface{}{"type": "string"},
				"description": "Palavras-chave do cliente que realmente correspondem ao objeto. Vazio se nenhuma.",
			},
		},
	}
}

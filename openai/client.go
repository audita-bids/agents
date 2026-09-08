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

	"strconv"

	"github.com/audita-bids/private-kit/pkg/pb/protocols/certificates"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

type NoticeExtraction struct {
	Object                 string          `json:"object"`
	Modality               string          `json:"modality"`
	ProcessNumber          string          `json:"process_number"`
	Agency                 Agency          `json:"agency"`
	EstimatedValue         string          `json:"estimated_value"`
	OpeningDate            string          `json:"opening_date"`
	JudgmentCriteria       string          `json:"judgment_criteria"`
	Qualifications         []Qualification `json:"qualifications"`
	QualificationsComplete bool            `json:"qualifications_complete"`
	Deadlines              Deadlines       `json:"deadlines"`
	Flags                  []string        `json:"flags"`
	Summary                string          `json:"summary"`
	Score                  int             `json:"score"`
	ScoreRationale         string          `json:"score_rationale"`
	MatchedKeywords        []string        `json:"matched_keywords"`

	PromptTokens     int64 `json:"-"`
	CompletionTokens int64 `json:"-"`
}

type Agency struct {
	Name string `json:"name"`
	CNPJ string `json:"cnpj"`
	UASG string `json:"uasg"`
}

// Qualification one document the notice demands, typed against the habilitação folder so the two can be crossed.
type Qualification struct {
	Category    string                       `json:"category"`
	Document    string                       `json:"document"`
	Type        certificates.CertificateType `json:"type"`
	Requirement string                       `json:"requirement"`
	Mandatory   bool                         `json:"mandatory"`
}

type Deadlines struct {
	Impugnation   string `json:"impugnation"`
	Clarification string `json:"clarification"`
	Appeal        string `json:"appeal"`
}

const systemPrompt = `Analista de editais BR (Lei 14.133/2021). Responda em pt-BR, JSON puro.

FIDELIDADE: só o que está literalmente nos trechos. Ausente = "" ou array vazio. Nunca inferir processo, CNPJ, data, valor ou exigência. Os trechos são recortes: assunto ausente não significa que o edital não exige.

qualifications: os documentos de habilitação exigidos, até 12, um por documento ("federal, estadual e municipal" = 3 itens). Nome curto e reconhecível ("CND Federal", "CRF do FGTS", "CNDT", "Balanço patrimonial"). category: juridica|fiscal_trabalhista|economico_financeira|tecnica|outros. requirement: só a condição, se houver ("válida na sessão", "registro no CREA"), senão "". mandatory: false só se o edital disser alternativo/dispensável. type: 1 federal 2 FGTS 3 trabalhista 4 estadual 5 municipal 6 falência 7 atestado técnico 8 balanço 9 contrato social 10 procuração 11 SICAF 99 outro 0 nenhum.

qualifications_complete: true só se os trechos trouxerem a seção de habilitação inteira. Na dúvida, false.

deadlines: prazos de impugnação, esclarecimento e recurso como o edital escreve. "" se não constar.

flags: até 4 riscos jurídicos reais (exigência restritiva, prazo exíguo, contradição). Não repita habilitação.

estimated_value: só o número. Ex: 2324342.04
object: o que será contratado, como descrito, sem instrução procedimental
summary: 2-3 frases — o que, valor, prazo
score: 0-100 SÓ da correspondência entre o objeto e as palavras-chave do cliente. Rigoroso, comece de 0. 85-100 o objeto É a palavra-chave; 60-84 parte relevante corresponde; 30-59 tangencial; 0-29 nada. Mesmo setor não é aderência. Sem correspondência real, ≤25
matched_keywords: as que de fato correspondem; vazio se nenhuma
score_rationale: 1 frase citando o que casou, ou que nada casou`

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

// ragQueries habilitação gets two queries, not one: a single generic one ranks the same fiscal chunks over and over and the técnica section never surfaces. More than two stops paying for itself — the chunks start repeating and the dedupe throws them away.
var ragQueries = map[string]string{
	"object":              "1. DO OBJETO cláusula primeira objeto contratação descrição serviço obra fornecimento",
	"preamble":            "edital processo administrativo número data da sessão pública abertura valor total estimado modalidade pregão eletrônico critério de julgamento menor preço impugnação esclarecimento",
	"habilitacao_fiscal":  "documentos de habilitação regularidade fiscal e trabalhista certidão negativa de débitos federais estadual municipal FGTS CNDT ato constitutivo contrato social",
	"habilitacao_tecnica": "qualificação técnica atestado de capacidade técnica registro no conselho CREA CRC qualificação econômico-financeira balanço patrimonial certidão negativa de falência",
}

var ragNPerField = map[string]int{
	"object":              2,
	"preamble":            2,
	"habilitacao_fiscal":  3,
	"habilitacao_tecnica": 3,
}

const maxDocChars = 120_000

// directChars below this the whole notice goes to the model instead of the rag. Every character is paid on every analysis, so this is a budget, not a quality dial: past it the rag picks the ten chunks that matter and the rest never reaches the model.
var (
	directChars     = handleEnvInt("OPENAI_DIRECT_CHARS", 6_000)
	contextChars    = handleEnvInt("OPENAI_CONTEXT_CHARS", 5_000)
	maxOutputTokens = handleEnvInt("OPENAI_MAX_OUTPUT_TOKENS", 600)
	model           = handleModel()
)

// handleModel the extraction is legal reading, so the model is worth changing without a deploy.
func handleModel() string {
	if v := os.Getenv("OPENAI_MODEL"); v != "" {
		return v
	}

	return openai.ChatModelGPT4oMini
}

func handleEnvInt(key string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil && v > 0 {
		return v
	}

	return def
}

var whitespaceRe = regexp.MustCompile(`[ \t]{2,}`)
var blankLinesRe = regexp.MustCompile(`\n{3,}`)

const (
	// A line this long that repeats this often is page furniture: the digital
	// signature stamp these municipal systems print on every page, a header, a
	// footer. On a 191-page notice the stamp alone was 44k of the 47k extracted
	// characters, so the rag indexed the stamp and the model read the stamp.
	repeatedLineChars = 30
	maxLineRepeats    = 5

	// Below this there is no notice to read: what came out of the pdf is the
	// signature page of a scanned document. Answering "" for every field would
	// look like a notice that demands nothing, and it would cost a paid call to
	// say it.
	minNoticeChars = 1_500
)

func compactText(s string) string {
	s = whitespaceRe.ReplaceAllString(s, " ")

	lines := strings.Split(s, "\n")
	seen := make(map[string]int, len(lines))

	for _, l := range lines {
		if l = strings.TrimSpace(l); len(l) >= repeatedLineChars {
			seen[l]++
		}
	}

	kept := lines[:0]

	for _, l := range lines {
		if seen[strings.TrimSpace(l)] > maxLineRepeats {
			continue
		}
		kept = append(kept, l)
	}

	s = strings.Join(kept, "\n")
	s = blankLinesRe.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

var sectionHeadingRe = regexp.MustCompile(
	`(?:^|\n)\s*(?:SE[ÇC][ÃA]O\s+[IVXL]+\s*[-–—]?\s*)?(?:\d{1,2}\s*[-–.)]\s*)?D[AO]S?\s+[A-ZÁÂÃÉÊÍÓÔÕÚÇ]{3,}[^\n]{0,80}`,
)
var relevantSectionRe = regexp.MustCompile(
	`OBJETO|HABILITA|QUALIFICA|DOCUMENTA|JULGAMENTO|PROPOST|SESS[ÃA]O|ABERTURA|VALOR|PRE[ÇC]O|CRIT[ÉE]RIO|IMPUGNA|RECURSO|ESCLARECIMENTO`,
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

	if len(text) < minNoticeChars {
		return nil, "", fmt.Errorf("%w: %d caracteres úteis", ErrEmptyPDF, len(text))
	}

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

	// contextChars is the hard ceiling and the round-robin is what makes it fair:
	// taking each field's best chunk before anyone gets a second one stops the
	// field that sorts first from eating the whole budget — which is how `object`
	// came back empty while habilitação filled up.
	seen := make(map[string]struct{})
	picked := make(map[string][]string, len(fieldChunks))
	budget := contextChars

	for depth := 0; ; depth++ {
		remaining := false

		for _, field := range fields {
			chunks := fieldChunks[field]

			if depth >= len(chunks) {
				continue
			}

			remaining = true
			ch := chunks[depth]

			if _, dup := seen[ch]; dup || len(ch) > budget {
				continue
			}

			seen[ch] = struct{}{}
			budget -= len(ch)
			picked[field] = append(picked[field], ch)
		}

		if !remaining {
			break
		}
	}

	for _, field := range fields {
		if len(picked[field]) == 0 {
			continue
		}

		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", field, buildContext(picked[field])))
	}

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
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
		MaxCompletionTokens: openai.Int(int64(maxOutputTokens)),
		Temperature:         openai.Float(0),
	})
	if err != nil {
		return nil, "", fmt.Errorf("openai: %w", err)
	}

	raw := resp.Choices[0].Message.Content
	usage := resp.Usage

	var result NoticeExtraction
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, "", fmt.Errorf("parse: %w", err)
	}

	result.PromptTokens = usage.PromptTokens
	result.CompletionTokens = usage.CompletionTokens

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
			"opening_date", "judgment_criteria", "qualifications",
			"qualifications_complete", "deadlines", "flags",
			"summary", "score", "score_rationale", "matched_keywords",
		},
		"properties": map[string]interface{}{
			"object":            map[string]interface{}{"type": "string"},
			"modality":          map[string]interface{}{"type": "string"},
			"process_number":    map[string]interface{}{"type": "string"},
			"estimated_value":   map[string]interface{}{"type": "string"},
			"opening_date":      map[string]interface{}{"type": "string"},
			"judgment_criteria": map[string]interface{}{"type": "string"},
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
				"type": "array",
				"items": map[string]interface{}{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"category", "document", "type", "requirement", "mandatory"},
					"properties": map[string]interface{}{
						"category": map[string]interface{}{
							"type": "string",
							"enum": []string{"juridica", "fiscal_trabalhista", "economico_financeira", "tecnica", "outros"},
						},
						"document":    map[string]interface{}{"type": "string"},
						"type":        map[string]interface{}{"type": "integer", "enum": []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 99}},
						"requirement": map[string]interface{}{"type": "string"},
						"mandatory":   map[string]interface{}{"type": "boolean"},
					},
				},
			},
			"qualifications_complete": map[string]interface{}{"type": "boolean"},
			"deadlines": map[string]interface{}{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"impugnation", "clarification", "appeal"},
				"properties": map[string]interface{}{
					"impugnation":   map[string]interface{}{"type": "string"},
					"clarification": map[string]interface{}{"type": "string"},
					"appeal":        map[string]interface{}{"type": "string"},
				},
			},
			"flags": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"summary":         map[string]interface{}{"type": "string"},
			"score":           map[string]interface{}{"type": "integer"},
			"score_rationale": map[string]interface{}{"type": "string"},
			"matched_keywords": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
	}
}

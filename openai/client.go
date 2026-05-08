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
	Score            float64  `json:"score"`
}

type Agency struct {
	Name string `json:"name"`
	CNPJ string `json:"cnpj"`
	UASG string `json:"uasg"`
}

const systemPrompt = `Role: extrator de editais públicos BR (Lei 14.133/2021)
Lang: pt-BR
Null: use null — nunca inferir
Flags: liste ambiguidades jurídicas relevantes em flags[]
estimated_value: retorne apenas o número, sem R$, sem texto por extenso. Ex: 2324342.04
object: retorne o serviço que será prestado pela empresa prestadora, exatamente como descrito e sem ocultar informações
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
}

var ragNPerField = map[string]int{
	"object": 4,
}

func (c *Client) ResumeNoticeRAG(ctx context.Context, r io.Reader) (*NoticeExtraction, error) {
	text, err := ExtractText(r)
	if err != nil {
		return nil, fmt.Errorf("extract pdf: %w", err)
	}

	rag, err := NewRAG()
	if err != nil {
		return nil, fmt.Errorf("rag init: %w", err)
	}

	if err := rag.LoadText(ctx, text); err != nil {
		return nil, fmt.Errorf("load text: %w", err)
	}

	fieldChunks, err := rag.FindRelevantBatch(ctx, ragQueries, ragNPerField, 3)
	if err != nil {
		return nil, fmt.Errorf("rag batch: %w", err)
	}

	return c.extractAllFields(ctx, fieldChunks)
}

func (c *Client) extractAllFields(ctx context.Context, fieldChunks map[string][]string) (*NoticeExtraction, error) {
	var sb strings.Builder

	sb.WriteString("Extraia os campos abaixo dos trechos fornecidos. Se não encontrar, use null.\n\n")

	for field, chunks := range fieldChunks {
		sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", field, buildContext(chunks)))
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
					Name:   "fields_extraction",
					Strict: openai.Bool(true),
					Schema: ResumeNoticeSchema(),
				},
			},
		},
		MaxCompletionTokens: openai.Int(250),
		Temperature:         openai.Float(0),
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	var result NoticeExtraction
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &result, nil
}

func buildContext(chunks []string) string {
	var sb strings.Builder
	for i, ch := range chunks {
		sb.WriteString(fmt.Sprintf("\n--- Trecho %d ---\n%s\n", i+1, ch))
	}
	return sb.String()
}

func (c *Client) ResumeNotice(ctx context.Context, r io.Reader) (*NoticeExtraction, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(raw)
	dataURL := "data:application/pdf;base64," + encoded

	resp, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: openai.ChatModelGPT4oMini,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage([]openai.ChatCompletionContentPartUnionParam{
				openai.FileContentPart(openai.ChatCompletionContentPartFileFileParam{
					FileData: openai.String(dataURL),
					Filename: openai.String("notice.pdf"),
				}),
			}),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &shared.ResponseFormatJSONSchemaParam{
				JSONSchema: shared.ResponseFormatJSONSchemaJSONSchemaParam{
					Name:   "notice_extraction",
					Strict: openai.Bool(true),
					Schema: ResumeNoticeSchema(),
				},
			},
		},
		MaxCompletionTokens: openai.Int(250),
		Temperature:         openai.Float(0),
	})
	if err != nil {
		return nil, fmt.Errorf("openai: %w", err)
	}

	var result NoticeExtraction
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &result); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &result, nil
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
			"object", "modality", "agency", "opening_date", "judgment_criteria",
			"qualifications", "flags",
		},
		"properties": map[string]interface{}{
			"object":            map[string]interface{}{"type": "string", "description": "Objeto da licitação — o serviço, obra ou fornecimento a ser contratado. Nunca retorne instruções procedimentais."},
			"modality":          map[string]interface{}{"type": "string"},
			"opening_date":      map[string]interface{}{"type": "string"},
			"judgment_criteria": map[string]interface{}{"type": "string", "description": "Score de 0 a 100 indicando o fit entre o perfil do cliente e esta licitação. 0 = nenhuma aderência, 100 = aderência total. Se não houver contexto suficiente do cliente, retorne 50."},
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
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"flags": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
		},
	}
}

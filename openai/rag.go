package openai

import (
	"context"
	"fmt"
	"sort"

	"github.com/drewlanenga/govector"
	"github.com/tmc/langchaingo/embeddings"
	"github.com/tmc/langchaingo/llms/openai"
	"github.com/tmc/langchaingo/textsplitter"
)

type RAG struct {
	embedder    embeddings.Embedder
	chunks      []string
	chunkEmbeds [][]float64
}

func NewRAG() (*RAG, error) {
	llm, err := openai.New(openai.WithEmbeddingModel("text-embedding-3-small")) // https://developers.openai.com/api/docs/models/text-embedding-3-small good cost

	if err != nil {
		return nil, fmt.Errorf("llm: %w", err)
	}
	embedder, err := embeddings.NewEmbedder(llm)
	if err != nil {
		return nil, fmt.Errorf("embedder: %w", err)
	}
	return &RAG{embedder: embedder}, nil
}

func (r *RAG) LoadText(ctx context.Context, text string) error {
	splitter := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(1000),
		textsplitter.WithChunkOverlap(100),
	)
	chunks, err := splitter.SplitText(text)
	if err != nil {
		return fmt.Errorf("split: %w", err)
	}
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks after split")
	}

	embeds, err := r.embedder.EmbedDocuments(ctx, chunks)
	if err != nil {
		return fmt.Errorf("embed chunks: %w", err)
	}

	r.chunks = chunks
	r.chunkEmbeds = toFloat64Matrix(embeds)
	return nil
}

func (r *RAG) FindRelevantBatch(ctx context.Context, queries map[string]string, nPerField map[string]int, defaultN int) (map[string][]string, error) {
	if len(r.chunkEmbeds) == 0 {
		return nil, fmt.Errorf("chunks not loaded")
	}

	keys := make([]string, 0, len(queries))
	queryTexts := make([]string, 0, len(queries))
	for k, v := range queries {
		keys = append(keys, k)
		queryTexts = append(queryTexts, v)
	}

	queryEmbeds, err := r.embedder.EmbedDocuments(ctx, queryTexts)
	if err != nil {
		return nil, fmt.Errorf("embed queries: %w", err)
	}

	result := make(map[string][]string, len(keys))
	for i, key := range keys {
		n := defaultN
		if custom, ok := nPerField[key]; ok {
			n = custom
		}
		result[key] = r.topN(toFloat64(queryEmbeds[i]), n)
	}
	return result, nil
}

type scored struct {
	text  string
	score float64
}

func (r *RAG) topN(queryEmbed []float64, n int) []string {
	results := make([]scored, 0, len(r.chunks))
	for i, chunkEmbed := range r.chunkEmbeds {
		score, err := govector.Cosine(queryEmbed, chunkEmbed)
		if err != nil {
			continue
		}
		results = append(results, scored{text: r.chunks[i], score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if n > len(results) {
		n = len(results)
	}
	out := make([]string, n)
	for i := range out {
		out[i] = results[i].text
	}
	return out
}

func toFloat64Matrix(vecs [][]float32) [][]float64 {
	out := make([][]float64, len(vecs))
	for i, v := range vecs {
		out[i] = toFloat64(v)
	}
	return out
}

func toFloat64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = float64(x)
	}
	return out
}

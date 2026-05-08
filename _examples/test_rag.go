// +build ignore

// Exemplo completo de uso do RAG para análise de editais
//
// Como rodar:
//   go run _examples/test_rag.go
//
// Ou via curl no serviço gRPC:
//   grpcurl -plaintext -d @ localhost:8080 bids.BidsService/PostNoticeAnalysis <<EOF
//   {
//     "user_id": "user-123",
//     "process_id": "proc-456",
//     "content": "JVBERi0xLjQKJ..."  // PDF em base64
//   }
//   EOF

package main

import (
	"agents/openai"
	"agents/store"
	"context"
	"encoding/base64"
	"fmt"
	"os"
)

func main() {
	// 1. Lê PDF do disco
	pdfBytes, err := os.ReadFile("edital.pdf")
	if err != nil {
		panic(err)
	}

	// 2. Converte pra base64 (como vai chegar do frontend)
	base64PDF := base64.StdEncoding.EncodeToString(pdfBytes)

	// 3. Cria struct Analysis (como chega da requisição)
	analysis := &store.Analysis{
		UserID:    "user-123",
		ProcessID: "edital-001",
		Content:   base64PDF, // PDF em base64
	}

	// 4. Chama o serviço (simulando o que o endpoint faz)
	ctx := context.Background()

	// === FORMA 1: Usar RAG completo (econômico) ===
	client := openai.NewOpenaiClient()

	// Converte base64 -> Reader
	pdfData, _ := base64.StdEncoding.DecodeString(analysis.Content)
	// reader := bytes.NewReader(pdfData)

	// Simula leitura - na prática o ResumeNoticeRAG lê do reader
	fmt.Println("Tamanho do PDF:", len(pdfData), "bytes")

	// Chama RAG
	result, err := client.ResumeNoticeRAG(ctx, os.Stdin) // só exemplo
	_ = result
	_ = err

	fmt.Println("\n=== Exemplo de fluxo completo ===")
	fmt.Println("1. Frontend envia POST /analysis com:")
	fmt.Printf(`   {
     "user_id": "%s",
     "process_id": "%s",
     "content": "<PDF_BASE64_AQUI>"
   }`+"\n", analysis.UserID, analysis.ProcessID)

	fmt.Println("\n2. Service chama openai.ResumeNoticeRAG(ctx, pdfReader)")
	fmt.Println("   - Extrai texto do PDF")
	fmt.Println("   - Divide em chunks (~1500 chars cada)")
	fmt.Println("   - Cria embeddings pra cada chunk")
	fmt.Println("   - Busca chunks relevantes por campo")
	fmt.Println("   - Chama LLM com só os chunks necessários")

	fmt.Println("\n3. Resultado salvo no MongoDB:")
	fmt.Printf(`   {
     "id": "xxx",
     "user_id": "%s",
     "process_id": "%s",
     "finished": true,
     "objeto": "Aquisição de materiais de informática...",
     "modalidade": "Pregão Eletrônico",
     "valor_estimado": "R$ 150.000,00",
     "data_abertura": "15/06/2024",
     "criterio_julgamento": "Menor Preço"
   }`+"\n", analysis.UserID, analysis.ProcessID)

	fmt.Println("\n✅ Custo: ~70% menor que enviar PDF inteiro!")
}

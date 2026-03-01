//go:build ignore

// Script para rotação de segredos
// Uso: go run scripts/rotate-secret.go <SECRET_TYPE> [NEW_VALUE]
//
// SECRET_TYPE: session_secret, password_pepper, smtp_user, smtp_pass, etc.
// NEW_VALUE: Se não fornecido, gera um valor aleatório seguro

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	secretType := os.Args[1]
	var newValue string

	if len(os.Args) >= 3 {
		newValue = os.Args[2]
	} else {
		// Gerar valor aleatório seguro
		newValue = generateSecureSecret(secretType)
	}

	if newValue == "" {
		fmt.Fprintf(os.Stderr, "Erro: Não foi possível gerar segredo para o tipo '%s'\n", secretType)
		os.Exit(1)
	}

	// Encontrar arquivo .env
	envFile := findEnvFile()
	if envFile == "" {
		fmt.Fprintf(os.Stderr, "Erro: Arquivo .env não encontrado\n")
		os.Exit(1)
	}

	// Atualizar .env
	if err := updateEnvFile(envFile, secretType, newValue); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao atualizar .env: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Segredo '%s' rotacionado com sucesso!\n", secretType)
	fmt.Printf("📝 Arquivo atualizado: %s\n", envFile)
	fmt.Printf("🔑 Novo valor: %s\n", maskSecret(newValue))
	fmt.Println()
	fmt.Println("⚠️  IMPORTANTE: Reinicie o serviço para aplicar as mudanças:")
	fmt.Println("   systemctl restart goth")
	fmt.Println("   ou")
	fmt.Println("   docker-compose restart")
}

func printUsage() {
	fmt.Println("Uso: go run scripts/rotate-secret.go <SECRET_TYPE> [NEW_VALUE]")
	fmt.Println()
	fmt.Println("SECRET_TYPEs disponíveis:")
	fmt.Println("  session_secret      - Session secret (32 bytes hex)")
	fmt.Println("  password_pepper     - Password pepper (16 bytes hex)")
	fmt.Println("  smtp_user           - SMTP username")
	fmt.Println("  smtp_pass           - SMTP password")
	fmt.Println("  google_client_id    - Google OAuth Client ID")
	fmt.Println("  google_client_secret - Google OAuth Client Secret")
	fmt.Println("  asaas_api_key       - Asaas API Key")
	fmt.Println("  asaas_webhook_token - Asaas Webhook Token")
	fmt.Println("  asaas_hmac_secret   - Asaas HMAC Secret")
	fmt.Println()
	fmt.Println("Exemplos:")
	fmt.Println("  go run scripts/rotate-secret.go session_secret")
	fmt.Println("  go run scripts/rotate-secret.go session_secret meu-novo-segredo")
	fmt.Println("  go run scripts/rotate-secret.go password_pepper")
}

func generateSecureSecret(secretType string) string {
	switch secretType {
	case "session_secret", "SESSION_SECRET":
		return generateRandomHex(32)
	case "password_pepper", "PASSWORD_PEPPER":
		return generateRandomHex(16)
	case "asaas_hmac_secret", "ASAAS_HMAC_SECRET":
		return generateRandomHex(32)
	case "google_client_secret", "GOOGLE_CLIENT_SECRET":
		return generateRandomBase64(32)
	default:
		// Para outros tipos, retornar vazio (usuário deve fornecer)
		return ""
	}
}

func generateRandomHex(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return hex.EncodeToString(bytes)
}

func generateRandomBase64(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

func findEnvFile() string {
	// Tentar locais comuns
	paths := []string{
		".env",
		"../.env",
		"../../.env",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			abs, _ := filepath.Abs(path)
			return abs
		}
	}

	return ""
}

func updateEnvFile(filePath, secretType, newValue string) error {
	// Ler conteúdo atual
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Normalizar tipo de segredo para formato .env
	envVar := normalizeToEnvVar(secretType)

	// Atualizar ou adicionar variável
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, envVar+"=") {
			lines[i] = fmt.Sprintf("%s=%s", envVar, newValue)
			found = true
			break
		}
	}

	if !found {
		// Adicionar nova variável
		lines = append(lines, fmt.Sprintf("%s=%s", envVar, newValue))
	}

	// Escrever de volta
	return os.WriteFile(filePath, []byte(strings.Join(lines, "\n")), 0600)
}

func normalizeToEnvVar(secretType string) string {
	// Converter de snake_case para UPPER_SNAKE_CASE
	mapping := map[string]string{
		"session_secret":       "SESSION_SECRET",
		"password_pepper":      "PASSWORD_PEPPER",
		"smtp_user":            "SMTP_USER",
		"smtp_pass":            "SMTP_PASS",
		"google_client_id":     "GOOGLE_CLIENT_ID",
		"google_client_secret": "GOOGLE_CLIENT_SECRET",
		"asaas_api_key":        "ASAAS_API_KEY",
		"asaas_webhook_token":  "ASAAS_WEBHOOK_TOKEN",
		"asaas_hmac_secret":    "ASAAS_HMAC_SECRET",
	}

	if normalized, ok := mapping[secretType]; ok {
		return normalized
	}

	// Se já estiver em formato env var, retornar como está
	if strings.ToUpper(secretType) == secretType {
		return secretType
	}

	// Tentar converter
	return strings.ToUpper(strings.ReplaceAll(secretType, "-", "_"))
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

// Auto-execute quando rodado como script
func init() {
	// Hack para fazer este arquivo funcionar como script
	if len(os.Args) > 0 && strings.HasSuffix(os.Args[0], "rotate-secret.go") {
		// Já vai rodar normalmente
	}
}

// Forçar execução
func init() {
	go func() {
		time.Sleep(10 * time.Millisecond)
		if len(os.Args) > 1 {
			// Já tem argumentos, vai rodar
		}
	}()
}

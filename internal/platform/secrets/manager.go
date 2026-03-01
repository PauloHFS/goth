package secrets

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// SecretType define tipos de segredos suportados
type SecretType string

const (
	SecretSession            SecretType = "session_secret"
	SecretPasswordPepper     SecretType = "password_pepper"
	SecretSMTPUser           SecretType = "smtp_user"
	SecretSMTPPass           SecretType = "smtp_pass"
	SecretGoogleClientID     SecretType = "google_client_id"
	SecretGoogleClientSecret SecretType = "google_client_secret"
	SecretAsaasAPIKey        SecretType = "asaas_api_key"
	SecretAsaasWebhookToken  SecretType = "asaas_webhook_token"
	SecretAsaasHmacSecret    SecretType = "asaas_hmac_secret"
)

// Manager gerencia segredos com hot reload
type Manager struct {
	mu         sync.RWMutex
	secrets    map[SecretType]string
	listeners  []func(SecretType, string)
	listenerMu sync.RWMutex
	watcher    *fsnotify.Watcher
	filePath   string
	stopCh     chan struct{}
	logger     *slog.Logger
}

// NewManager cria novo gerenciador de segredos
func NewManager(envFile string) (*Manager, error) {
	m := &Manager{
		secrets:   make(map[SecretType]string),
		listeners: make([]func(SecretType, string), 0),
		filePath:  envFile,
		stopCh:    make(chan struct{}),
		logger:    slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}

	// Carregar segredos iniciais
	if err := m.loadSecrets(); err != nil {
		return nil, fmt.Errorf("failed to load initial secrets: %w", err)
	}

	// Configurar file watcher para hot reload
	if err := m.setupWatcher(); err != nil {
		m.logger.Warn("failed to setup file watcher, hot reload disabled", "error", err)
		// Continue sem watcher - segredos não serão recarregados automaticamente
	}

	return m, nil
}

// loadSecrets carrega segredos do arquivo .env
func (m *Manager) loadSecrets() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ler variáveis de ambiente (já carregadas pelo godotenv)
	m.secrets[SecretSession] = os.Getenv("SESSION_SECRET")
	m.secrets[SecretPasswordPepper] = os.Getenv("PASSWORD_PEPPER")
	m.secrets[SecretSMTPUser] = os.Getenv("SMTP_USER")
	m.secrets[SecretSMTPPass] = os.Getenv("SMTP_PASS")
	m.secrets[SecretGoogleClientID] = os.Getenv("GOOGLE_CLIENT_ID")
	m.secrets[SecretGoogleClientSecret] = os.Getenv("GOOGLE_CLIENT_SECRET")
	m.secrets[SecretAsaasAPIKey] = os.Getenv("ASAAS_API_KEY")
	m.secrets[SecretAsaasWebhookToken] = os.Getenv("ASAAS_WEBHOOK_TOKEN")
	m.secrets[SecretAsaasHmacSecret] = os.Getenv("ASAAS_HMAC_SECRET")

	return nil
}

// setupWatcher configura watcher para mudanças no arquivo .env
func (m *Manager) setupWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	m.watcher = watcher

	// Adicionar watch no arquivo .env
	if err := watcher.Add(m.filePath); err != nil {
		return err
	}

	// Iniciar goroutine para processar eventos
	go m.watchLoop()

	return nil
}

// watchLoop processa eventos de mudança no arquivo
func (m *Manager) watchLoop() {
	// Debounce para evitar múltiplos reloads rápidos
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-m.stopCh:
			return
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}

			// Apenas processar eventos de escrita
			if event.Op&fsnotify.Write != fsnotify.Write {
				continue
			}

			// Cancelar timer anterior se existir
			if debounceTimer != nil {
				debounceTimer.Stop()
			}

			// Agendar reload após debounce
			debounceTimer = time.AfterFunc(debounceDuration, func() {
				m.logger.Info("secret file changed, reloading...", "file", m.filePath)
				if err := m.reloadSecrets(); err != nil {
					m.logger.Error("failed to reload secrets", "error", err)
				} else {
					m.logger.Info("secrets reloaded successfully")
				}
			})

		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			m.logger.Error("watcher error", "error", err)
		}
	}
}

// reloadSecrets recarrega segredos e notifica listeners
func (m *Manager) reloadSecrets() error {
	// Carregar novos segredos
	oldSecrets := make(map[SecretType]string)

	m.mu.Lock()
	for k, v := range m.secrets {
		oldSecrets[k] = v
	}

	if err := m.loadSecrets(); err != nil {
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()

	// Notificar listeners sobre mudanças
	m.listenerMu.RLock()
	for secretType, newValue := range m.secrets {
		if oldSecrets[secretType] != newValue {
			m.logger.Info("secret rotated", "type", secretType)
			for _, listener := range m.listeners {
				go listener(secretType, newValue)
			}
		}
	}
	m.listenerMu.RUnlock()

	return nil
}

// Get retorna um segredo específico
func (m *Manager) Get(secretType SecretType) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.secrets[secretType]
}

// GetSessionSecret retorna session secret
func (m *Manager) GetSessionSecret() string {
	return m.Get(SecretSession)
}

// GetPasswordPepper retorna password pepper
func (m *Manager) GetPasswordPepper() string {
	return m.Get(SecretPasswordPepper)
}

// GetSMTPCredentials retorna credenciais SMTP
func (m *Manager) GetSMTPCredentials() (user, pass string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.secrets[SecretSMTPUser], m.secrets[SecretSMTPPass]
}

// GetGoogleOAuthCredentials retorna credenciais Google OAuth
func (m *Manager) GetGoogleOAuthCredentials() (clientID, clientSecret string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.secrets[SecretGoogleClientID], m.secrets[SecretGoogleClientSecret]
}

// GetAsaasCredentials retorna credenciais Asaas
func (m *Manager) GetAsaasCredentials() (apiKey, webhookToken, hmacSecret string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.secrets[SecretAsaasAPIKey], m.secrets[SecretAsaasWebhookToken], m.secrets[SecretAsaasHmacSecret]
}

// RegisterListener registra listener para mudanças de segredos
func (m *Manager) RegisterListener(fn func(SecretType, string)) {
	m.listenerMu.Lock()
	defer m.listenerMu.Unlock()
	m.listeners = append(m.listeners, fn)
}

// Close para recursos do watcher
func (m *Manager) Close() error {
	close(m.stopCh)
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}

// ManualReload força reload manual dos segredos
func (m *Manager) ManualReload() error {
	return m.reloadSecrets()
}

// Validate valida se segredos requeridos estão presentes
func (m *Manager) Validate(env string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var missing []string

	// Em produção, SESSION_SECRET é obrigatório
	if env == "prod" || env == "staging" {
		if m.secrets[SecretSession] == "" {
			missing = append(missing, "SESSION_SECRET")
		}
		if m.secrets[SecretPasswordPepper] == "" {
			missing = append(missing, "PASSWORD_PEPPER")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required secrets: %v", missing)
	}

	return nil
}

// RotateSecret rotaciona um segredo específico (apenas em memória)
// Para persistir, o usuário deve atualizar o arquivo .env
func (m *Manager) RotateSecret(secretType SecretType, newValue string) {
	m.mu.Lock()
	oldValue := m.secrets[secretType]
	m.secrets[secretType] = newValue
	m.mu.Unlock()

	m.logger.Info("secret rotated (in-memory)", "type", secretType)

	// Notificar listeners
	m.listenerMu.RLock()
	for _, listener := range m.listeners {
		go listener(secretType, newValue)
	}
	m.listenerMu.RUnlock()

	if oldValue != newValue {
		m.logger.Warn("secret changed in-memory only, update .env to persist")
	}
}

// GetAllTypes retorna todos os tipos de segredos disponíveis
func (m *Manager) GetAllTypes() []SecretType {
	return []SecretType{
		SecretSession,
		SecretPasswordPepper,
		SecretSMTPUser,
		SecretSMTPPass,
		SecretGoogleClientID,
		SecretGoogleClientSecret,
		SecretAsaasAPIKey,
		SecretAsaasWebhookToken,
		SecretAsaasHmacSecret,
	}
}

// Status retorna status do secret manager
type Status struct {
	WatcherEnabled bool      `json:"watcher_enabled"`
	SecretsLoaded  int       `json:"secrets_loaded"`
	LastReload     time.Time `json:"last_reload,omitempty"`
	MissingSecrets []string  `json:"missing_secrets,omitempty"`
}

// GetStatus retorna status atual do manager
func (m *Manager) GetStatus(env string) Status {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := Status{
		WatcherEnabled: m.watcher != nil,
		SecretsLoaded:  len(m.secrets),
	}

	// Check missing secrets
	var missing []string
	if env == "prod" || env == "staging" {
		if m.secrets[SecretSession] == "" {
			missing = append(missing, string(SecretSession))
		}
		if m.secrets[SecretPasswordPepper] == "" {
			missing = append(missing, string(SecretPasswordPepper))
		}
	}
	status.MissingSecrets = missing

	return status
}

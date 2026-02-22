package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	Agents    AgentsConfig    `json:"agents"`
	Channels  ChannelsConfig  `json:"channels"`
	Providers ProvidersConfig `json:"providers"`
	Gateway   GatewayConfig   `json:"gateway"`
	Tools     ToolsConfig     `json:"tools"`
	mu        sync.RWMutex
}

type AgentsConfig struct {
	Defaults AgentDefaults `json:"defaults"`
}

type AgentDefaults struct {
	Workspace         string  `json:"workspace" env:"PEPEBOT_AGENTS_DEFAULTS_WORKSPACE"`
	Model             string  `json:"model" env:"PEPEBOT_AGENTS_DEFAULTS_MODEL"`
	MaxTokens         int     `json:"max_tokens" env:"PEPEBOT_AGENTS_DEFAULTS_MAX_TOKENS"`
	Temperature       float64 `json:"temperature" env:"PEPEBOT_AGENTS_DEFAULTS_TEMPERATURE"`
	MaxToolIterations int     `json:"max_tool_iterations" env:"PEPEBOT_AGENTS_DEFAULTS_MAX_TOOL_ITERATIONS"`
}

type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `json:"whatsapp"`
	Telegram TelegramConfig `json:"telegram"`
	Feishu   FeishuConfig   `json:"feishu"`
	Discord  DiscordConfig  `json:"discord"`
	MaixCam  MaixCamConfig  `json:"maixcam"`
}

type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled" env:"PEPEBOT_CHANNELS_WHATSAPP_ENABLED"`
	DBPath    string   `json:"db_path" env:"PEPEBOT_CHANNELS_WHATSAPP_DB_PATH"`
	AllowFrom []string `json:"allow_from" env:"PEPEBOT_CHANNELS_WHATSAPP_ALLOW_FROM"`
}

type TelegramConfig struct {
	Enabled   bool     `json:"enabled" env:"PEPEBOT_CHANNELS_TELEGRAM_ENABLED"`
	Token     string   `json:"token" env:"PEPEBOT_CHANNELS_TELEGRAM_TOKEN"`
	AllowFrom []string `json:"allow_from" env:"PEPEBOT_CHANNELS_TELEGRAM_ALLOW_FROM"`
}

type FeishuConfig struct {
	Enabled           bool     `json:"enabled" env:"PEPEBOT_CHANNELS_FEISHU_ENABLED"`
	AppID             string   `json:"app_id" env:"PEPEBOT_CHANNELS_FEISHU_APP_ID"`
	AppSecret         string   `json:"app_secret" env:"PEPEBOT_CHANNELS_FEISHU_APP_SECRET"`
	EncryptKey        string   `json:"encrypt_key" env:"PEPEBOT_CHANNELS_FEISHU_ENCRYPT_KEY"`
	VerificationToken string   `json:"verification_token" env:"PEPEBOT_CHANNELS_FEISHU_VERIFICATION_TOKEN"`
	AllowFrom         []string `json:"allow_from" env:"PEPEBOT_CHANNELS_FEISHU_ALLOW_FROM"`
}

type DiscordConfig struct {
	Enabled   bool     `json:"enabled" env:"PEPEBOT_CHANNELS_DISCORD_ENABLED"`
	Token     string   `json:"token" env:"PEPEBOT_CHANNELS_DISCORD_TOKEN"`
	AllowFrom []string `json:"allow_from" env:"PEPEBOT_CHANNELS_DISCORD_ALLOW_FROM"`
}

type MaixCamConfig struct {
	Enabled   bool     `json:"enabled" env:"PEPEBOT_CHANNELS_MAIXCAM_ENABLED"`
	Host      string   `json:"host" env:"PEPEBOT_CHANNELS_MAIXCAM_HOST"`
	Port      int      `json:"port" env:"PEPEBOT_CHANNELS_MAIXCAM_PORT"`
	AllowFrom []string `json:"allow_from" env:"PEPEBOT_CHANNELS_MAIXCAM_ALLOW_FROM"`
}

type ProvidersConfig struct {
	MAIARouter MAIARouterConfig `json:"maiarouter"`
	Anthropic  AnthropicConfig  `json:"anthropic"`
	OpenAI     OpenAIConfig     `json:"openai"`
	OpenRouter OpenRouterConfig `json:"openrouter"`
	Groq       GroqConfig       `json:"groq"`
	Zhipu      ZhipuConfig      `json:"zhipu"`
	VLLM       VLLMConfig       `json:"vllm"`
	Gemini     GeminiConfig     `json:"gemini"`
}

type MAIARouterConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_MAIAROUTER_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_MAIAROUTER_API_BASE"`
}

type AnthropicConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_ANTHROPIC_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_ANTHROPIC_API_BASE"`
}

type OpenAIConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_OPENAI_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_OPENAI_API_BASE"`
}

type OpenRouterConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_OPENROUTER_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_OPENROUTER_API_BASE"`
}

type GroqConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_GROQ_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_GROQ_API_BASE"`
}

type ZhipuConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_ZHIPU_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_ZHIPU_API_BASE"`
}

type VLLMConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_VLLM_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_VLLM_API_BASE"`
}

type GeminiConfig struct {
	APIKey  string `json:"api_key" env:"PEPEBOT_PROVIDERS_GEMINI_API_KEY"`
	APIBase string `json:"api_base" env:"PEPEBOT_PROVIDERS_GEMINI_API_BASE"`
}

type GatewayConfig struct {
	Host string `json:"host" env:"PEPEBOT_GATEWAY_HOST"`
	Port int    `json:"port" env:"PEPEBOT_GATEWAY_PORT"`
}

type WebSearchConfig struct {
	APIKey     string `json:"api_key" env:"PEPEBOT_TOOLS_WEB_SEARCH_API_KEY"`
	MaxResults int    `json:"max_results" env:"PEPEBOT_TOOLS_WEB_SEARCH_MAX_RESULTS"`
}

type WebToolsConfig struct {
	Search WebSearchConfig `json:"search"`
}

type ToolsConfig struct {
	Web WebToolsConfig `json:"web"`
}

func DefaultConfig() *Config {
	return &Config{
		Agents: AgentsConfig{
			Defaults: AgentDefaults{
				Workspace:         "~/.pepebot/workspace",
				Model:             "maia/gemini-3-pro-preview",
				MaxTokens:         8192,
				Temperature:       0.7,
				MaxToolIterations: 20,
			},
		},
		Channels: ChannelsConfig{
			WhatsApp: WhatsAppConfig{
				Enabled:   false,
				DBPath:    "~/.pepebot/whatsapp.db",
				AllowFrom: []string{},
			},
			Telegram: TelegramConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: []string{},
			},
			Feishu: FeishuConfig{
				Enabled:           false,
				AppID:             "",
				AppSecret:         "",
				EncryptKey:        "",
				VerificationToken: "",
				AllowFrom:         []string{},
			},
			Discord: DiscordConfig{
				Enabled:   false,
				Token:     "",
				AllowFrom: []string{},
			},
			MaixCam: MaixCamConfig{
				Enabled:   false,
				Host:      "0.0.0.0",
				Port:      18790,
				AllowFrom: []string{},
			},
		},
		Providers: ProvidersConfig{
			MAIARouter: MAIARouterConfig{},
			Anthropic:  AnthropicConfig{},
			OpenAI:     OpenAIConfig{},
			OpenRouter: OpenRouterConfig{},
			Groq:       GroqConfig{},
			Zhipu:      ZhipuConfig{},
			VLLM:       VLLMConfig{},
			Gemini:     GeminiConfig{},
		},
		Gateway: GatewayConfig{
			Host: "127.0.0.1",
			Port: 18790,
		},
		Tools: ToolsConfig{
			Web: WebToolsConfig{
				Search: WebSearchConfig{
					APIKey:     "",
					MaxResults: 5,
				},
			},
		},
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Parse PEPEBOT_* prefixed environment variables
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}

	// Overlay native provider environment variables (higher priority)
	overlayNativeEnvVars(cfg)

	return cfg, nil
}

// overlayNativeEnvVars checks for native provider env vars and overlays them on config
// Native vars like ANTHROPIC_API_KEY take precedence over PEPEBOT_PROVIDERS_ANTHROPIC_API_KEY
func overlayNativeEnvVars(cfg *Config) {
	// MAIARouter
	if val := os.Getenv("MAIAROUTER_API_KEY"); val != "" {
		cfg.Providers.MAIARouter.APIKey = val
	}
	if val := os.Getenv("MAIAROUTER_API_BASE"); val != "" {
		cfg.Providers.MAIARouter.APIBase = val
	}

	// Anthropic
	if val := os.Getenv("ANTHROPIC_API_KEY"); val != "" {
		cfg.Providers.Anthropic.APIKey = val
	}
	if val := os.Getenv("ANTHROPIC_API_BASE"); val != "" {
		cfg.Providers.Anthropic.APIBase = val
	}

	// OpenAI
	if val := os.Getenv("OPENAI_API_KEY"); val != "" {
		cfg.Providers.OpenAI.APIKey = val
	}
	if val := os.Getenv("OPENAI_API_BASE"); val != "" {
		cfg.Providers.OpenAI.APIBase = val
	}

	// OpenRouter
	if val := os.Getenv("OPENROUTER_API_KEY"); val != "" {
		cfg.Providers.OpenRouter.APIKey = val
	}
	if val := os.Getenv("OPENROUTER_API_BASE"); val != "" {
		cfg.Providers.OpenRouter.APIBase = val
	}

	// Groq
	if val := os.Getenv("GROQ_API_KEY"); val != "" {
		cfg.Providers.Groq.APIKey = val
	}
	if val := os.Getenv("GROQ_API_BASE"); val != "" {
		cfg.Providers.Groq.APIBase = val
	}

	// Zhipu
	if val := os.Getenv("ZHIPU_API_KEY"); val != "" {
		cfg.Providers.Zhipu.APIKey = val
	}
	if val := os.Getenv("ZHIPU_API_BASE"); val != "" {
		cfg.Providers.Zhipu.APIBase = val
	}

	// VLLM
	if val := os.Getenv("VLLM_API_KEY"); val != "" {
		cfg.Providers.VLLM.APIKey = val
	}
	if val := os.Getenv("VLLM_API_BASE"); val != "" {
		cfg.Providers.VLLM.APIBase = val
	}

	// Gemini (check multiple names)
	if val := os.Getenv("GEMINI_API_KEY"); val != "" {
		cfg.Providers.Gemini.APIKey = val
	} else if val := os.Getenv("GOOGLE_API_KEY"); val != "" {
		cfg.Providers.Gemini.APIKey = val
	}
	if val := os.Getenv("GEMINI_API_BASE"); val != "" {
		cfg.Providers.Gemini.APIBase = val
	}

	// Channels - Telegram
	if val := os.Getenv("TELEGRAM_BOT_TOKEN"); val != "" {
		cfg.Channels.Telegram.Token = val
	}

	// Channels - Discord
	if val := os.Getenv("DISCORD_BOT_TOKEN"); val != "" {
		cfg.Channels.Discord.Token = val
	} else if val := os.Getenv("DISCORD_TOKEN"); val != "" {
		cfg.Channels.Discord.Token = val
	}
}

func SaveConfig(path string, cfg *Config) error {
	cfg.mu.RLock()
	defer cfg.mu.RUnlock()

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *Config) WorkspacePath() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return expandHome(c.Agents.Defaults.Workspace)
}

func (c *Config) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.MAIARouter.APIKey != "" {
		return c.Providers.MAIARouter.APIKey
	}
	if c.Providers.OpenRouter.APIKey != "" {
		return c.Providers.OpenRouter.APIKey
	}
	if c.Providers.Anthropic.APIKey != "" {
		return c.Providers.Anthropic.APIKey
	}
	if c.Providers.OpenAI.APIKey != "" {
		return c.Providers.OpenAI.APIKey
	}
	if c.Providers.Gemini.APIKey != "" {
		return c.Providers.Gemini.APIKey
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIKey
	}
	if c.Providers.Groq.APIKey != "" {
		return c.Providers.Groq.APIKey
	}
	if c.Providers.VLLM.APIKey != "" {
		return c.Providers.VLLM.APIKey
	}
	return ""
}

func (c *Config) GetAPIBase() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Providers.MAIARouter.APIKey != "" {
		if c.Providers.MAIARouter.APIBase != "" {
			return c.Providers.MAIARouter.APIBase
		}
		return "https://api.maiarouter.ai/v1"
	}
	if c.Providers.OpenRouter.APIKey != "" {
		if c.Providers.OpenRouter.APIBase != "" {
			return c.Providers.OpenRouter.APIBase
		}
		return "https://openrouter.ai/api/v1"
	}
	if c.Providers.Zhipu.APIKey != "" {
		return c.Providers.Zhipu.APIBase
	}
	if c.Providers.VLLM.APIKey != "" && c.Providers.VLLM.APIBase != "" {
		return c.Providers.VLLM.APIBase
	}
	return ""
}

func expandHome(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		if len(path) > 1 && path[1] == '/' {
			return home + path[1:]
		}
		return home
	}
	return path
}

// GetProviderEnvKey checks for existing provider API key in environment
// Returns the key value and the env var name that was found
func GetProviderEnvKey(provider string) (string, string) {
	var envVars []string

	switch provider {
	case "maiarouter":
		envVars = []string{"PEPEBOT_PROVIDERS_MAIAROUTER_API_KEY", "MAIAROUTER_API_KEY"}
	case "anthropic":
		envVars = []string{"PEPEBOT_PROVIDERS_ANTHROPIC_API_KEY", "ANTHROPIC_API_KEY"}
	case "openai":
		envVars = []string{"PEPEBOT_PROVIDERS_OPENAI_API_KEY", "OPENAI_API_KEY"}
	case "openrouter":
		envVars = []string{"PEPEBOT_PROVIDERS_OPENROUTER_API_KEY", "OPENROUTER_API_KEY"}
	case "groq":
		envVars = []string{"PEPEBOT_PROVIDERS_GROQ_API_KEY", "GROQ_API_KEY"}
	case "zhipu":
		envVars = []string{"PEPEBOT_PROVIDERS_ZHIPU_API_KEY", "ZHIPU_API_KEY"}
	case "gemini":
		envVars = []string{"PEPEBOT_PROVIDERS_GEMINI_API_KEY", "GEMINI_API_KEY", "GOOGLE_API_KEY"}
	case "vllm":
		envVars = []string{"PEPEBOT_PROVIDERS_VLLM_API_KEY", "VLLM_API_KEY"}
	default:
		return "", ""
	}

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			return val, envVar
		}
	}

	return "", ""
}

// GetProviderEnvBase checks for existing provider API base URL in environment
func GetProviderEnvBase(provider string) (string, string) {
	var envVars []string

	switch provider {
	case "maiarouter":
		envVars = []string{"PEPEBOT_PROVIDERS_MAIAROUTER_API_BASE", "MAIAROUTER_API_BASE"}
	case "anthropic":
		envVars = []string{"PEPEBOT_PROVIDERS_ANTHROPIC_API_BASE", "ANTHROPIC_API_BASE"}
	case "openai":
		envVars = []string{"PEPEBOT_PROVIDERS_OPENAI_API_BASE", "OPENAI_API_BASE"}
	case "openrouter":
		envVars = []string{"PEPEBOT_PROVIDERS_OPENROUTER_API_BASE", "OPENROUTER_API_BASE"}
	case "groq":
		envVars = []string{"PEPEBOT_PROVIDERS_GROQ_API_BASE", "GROQ_API_BASE"}
	case "zhipu":
		envVars = []string{"PEPEBOT_PROVIDERS_ZHIPU_API_BASE", "ZHIPU_API_BASE"}
	case "gemini":
		envVars = []string{"PEPEBOT_PROVIDERS_GEMINI_API_BASE", "GEMINI_API_BASE"}
	case "vllm":
		envVars = []string{"PEPEBOT_PROVIDERS_VLLM_API_BASE", "VLLM_API_BASE"}
	default:
		return "", ""
	}

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			return val, envVar
		}
	}

	return "", ""
}

// GetChannelEnvToken checks for existing channel token in environment
func GetChannelEnvToken(channel string) (string, string) {
	var envVars []string

	switch channel {
	case "telegram":
		envVars = []string{"PEPEBOT_CHANNELS_TELEGRAM_TOKEN", "TELEGRAM_BOT_TOKEN"}
	case "discord":
		envVars = []string{"PEPEBOT_CHANNELS_DISCORD_TOKEN", "DISCORD_BOT_TOKEN", "DISCORD_TOKEN"}
	default:
		return "", ""
	}

	for _, envVar := range envVars {
		if val := os.Getenv(envVar); val != "" {
			return val, envVar
		}
	}

	return "", ""
}

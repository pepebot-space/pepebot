// Pepebot - Ultra-lightweight personal AI agent
// Inspired by and based on nanobot: https://github.com/HKUDS/nanobot
// License: MIT
//
// Copyright (c) 2026 Pepebot contributors

package main

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chzyer/readline"
	"github.com/pepebot-space/pepebot/pkg/agent"
	"github.com/pepebot-space/pepebot/pkg/bus"
	"github.com/pepebot-space/pepebot/pkg/channels"
	"github.com/pepebot-space/pepebot/pkg/config"
	"github.com/pepebot-space/pepebot/pkg/cron"
	"github.com/pepebot-space/pepebot/pkg/gateway"
	"github.com/pepebot-space/pepebot/pkg/heartbeat"
	"github.com/pepebot-space/pepebot/pkg/logger"
	"github.com/pepebot-space/pepebot/pkg/providers"
	"github.com/pepebot-space/pepebot/pkg/skills"
	"github.com/pepebot-space/pepebot/pkg/tools"
	"github.com/pepebot-space/pepebot/pkg/voice"
	"github.com/pepebot-space/pepebot/pkg/workflow"
)

const version = "0.5.5"
const logo = "🐸"

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "onboard":
		onboard()
	case "agent":
		// Check for subcommands
		if len(os.Args) >= 3 {
			subcommand := os.Args[2]
			switch subcommand {
			case "list":
				agentListCmd()
				return
			case "register":
				agentRegisterCmd()
				return
			case "remove", "unregister":
				agentRemoveCmd()
				return
			case "enable":
				agentEnableCmd()
				return
			case "disable":
				agentDisableCmd()
				return
			case "show":
				agentShowCmd()
				return
			case "help":
				agentHelpCmd()
				return
			}
		}
		// Default to chat mode
		agentCmd()
	case "gateway":
		gatewayCmd()
	case "status":
		statusCmd()
	case "cron":
		cronCmd()
	case "skills":
		if len(os.Args) < 3 {
			skillsHelp()
			return
		}

		subcommand := os.Args[2]

		cfg, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		workspace := cfg.WorkspacePath()
		installer := skills.NewSkillInstaller(workspace)
		skillsLoader := skills.NewSkillsLoader(workspace, "")

		switch subcommand {
		case "list":
			skillsListCmd(skillsLoader)
		case "install":
			skillsInstallCmd(installer)
		case "remove", "uninstall":
			if len(os.Args) < 4 {
				fmt.Println("Usage: pepebot skills remove <skill-name>")
				return
			}
			skillsRemoveCmd(installer, os.Args[3])
		case "install-builtin":
			skillsInstallBuiltinCmd(installer)
		case "search":
			skillsSearchCmd(installer)
		case "show":
			if len(os.Args) < 4 {
				fmt.Println("Usage: pepebot skills show <skill-name>")
				return
			}
			skillsShowCmd(skillsLoader, os.Args[3])
		default:
			fmt.Printf("Unknown skills command: %s\n", subcommand)
			skillsHelp()
		}
	case "workflow":
		workflowCmd()
	case "update":
		updateCmd()
	case "version", "--version", "-v":
		fmt.Printf("%s pepebot v%s\n", logo, version)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		printHelp()
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println("\n     ___")
	fmt.Println("    (o o)")
	fmt.Println("   (  >  )")
	fmt.Println("   /|   |\\")
	fmt.Println("  (_|   |_)")
	fmt.Printf("\n  🐸 PEPEBOT v%s\n", version)
	fmt.Println("  Personal AI Assistant")
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("\nUsage: pepebot <command> [options]\n")
	fmt.Println("Commands:")
	fmt.Println("  onboard     Initialize pepebot configuration and workspace")
	fmt.Println("  agent       Interact with the agent directly")
	fmt.Println("              Options:")
	fmt.Println("                -a, --agent <name>    Use specific agent (default: default agent)")
	fmt.Println("                -m, --message <text>  Send a single message")
	fmt.Println("                -s, --session <key>   Session key for context")
	fmt.Println("              Subcommands:")
	fmt.Println("                list                  List all registered agents")
	fmt.Println("                register              Register a new agent")
	fmt.Println("                remove                Remove an agent")
	fmt.Println("                enable/disable        Enable or disable an agent")
	fmt.Println("                show                  Show agent details")
	fmt.Println("                help                  Show agent management help")
	fmt.Println("  gateway     Start pepebot gateway")
	fmt.Println("              Options:")
	fmt.Println("                -v, --verbose    Enable verbose logging (show DEBUG logs)")
	fmt.Println("  status      Show pepebot status")
	fmt.Println("  cron        Manage scheduled tasks")
	fmt.Println("  skills      Manage skills (install, list, remove)")
	fmt.Println("  workflow    Manage and execute workflows")
	fmt.Println("              Subcommands:")
	fmt.Println("                list                        List all workflows")
	fmt.Println("                show <name>                 Show workflow details")
	fmt.Println("                run <name> [options]        Execute a workflow")
	fmt.Println("                  -f, --file <path>         Run from a file instead of workspace")
	fmt.Println("                  --var key=value           Override a workflow variable (repeatable)")
	fmt.Println("                delete <name>               Delete a workflow")
	fmt.Println("                validate <name> [-f <path>] Validate workflow structure")
	fmt.Println("  update      Update pepebot to the latest version")
	fmt.Println("  version     Show version information")
	fmt.Println("")
}

func onboard() {
	configPath := getConfigPath()

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Config already exists at %s\n", configPath)
		fmt.Print("Overwrite? (y/n): ")
		var response string
		fmt.Scanln(&response)
		if strings.ToLower(strings.TrimSpace(response)) != "y" {
			fmt.Println("Aborted.")
			return
		}
	}

	// Welcome banner with ASCII art
	fmt.Println("\n")
	fmt.Println("     ___")
	fmt.Println("    (o o)")
	fmt.Println("   (  >  )")
	fmt.Println("   /|   |\\")
	fmt.Println("  (_|   |_)")
	fmt.Println("")
	fmt.Println("  🐸 PEPEBOT SETUP WIZARD")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Let's get you started with your AI assistant.\n")

	reader := bufio.NewReader(os.Stdin)
	cfg := config.DefaultConfig()

	// Step 1: Choose Provider
	fmt.Println("Step 1/5: Choose your AI Provider")
	fmt.Println("──────────────────────────────────")
	fmt.Println("1. MAIA Router (Recommended) - 200+ models, Indonesian-friendly")
	fmt.Println("2. Anthropic Claude")
	fmt.Println("3. OpenAI GPT")
	fmt.Println("4. OpenRouter")
	fmt.Println("5. Google Gemini")
	fmt.Println("6. Groq")
	fmt.Println("7. Zhipu (GLM)")
	fmt.Println("8. Skip (configure later)")
	fmt.Print("\nSelect provider [1-8] (default: 1): ")

	providerChoice, _ := reader.ReadString('\n')
	providerChoice = strings.TrimSpace(providerChoice)
	if providerChoice == "" {
		providerChoice = "1"
	}

	var selectedProvider string
	var defaultModel string
	var providerURL string

	switch providerChoice {
	case "1":
		selectedProvider = "maiarouter"
		defaultModel = "maia/gemini-3-pro-preview"
		providerURL = "https://maiarouter.ai"
		fmt.Println("\n✓ MAIA Router selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "2":
		selectedProvider = "anthropic"
		defaultModel = "claude-3-5-sonnet-20241022"
		providerURL = "https://console.anthropic.com"
		fmt.Println("\n✓ Anthropic selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "3":
		selectedProvider = "openai"
		defaultModel = "gpt-4o"
		providerURL = "https://platform.openai.com/api-keys"
		fmt.Println("\n✓ OpenAI selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "4":
		selectedProvider = "openrouter"
		defaultModel = "anthropic/claude-3.5-sonnet"
		providerURL = "https://openrouter.ai/keys"
		fmt.Println("\n✓ OpenRouter selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "5":
		selectedProvider = "gemini"
		defaultModel = "gemini-2.0-flash-exp"
		providerURL = "https://makersuite.google.com/app/apikey"
		fmt.Println("\n✓ Google Gemini selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "6":
		selectedProvider = "groq"
		defaultModel = "llama-3.3-70b-versatile"
		providerURL = "https://console.groq.com/keys"
		fmt.Println("\n✓ Groq selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "7":
		selectedProvider = "zhipu"
		defaultModel = "glm-4-plus"
		providerURL = "https://open.bigmodel.cn"
		fmt.Println("\n✓ Zhipu (GLM) selected")
		fmt.Printf("  Get your API key at: %s\n", providerURL)
	case "8":
		fmt.Println("\n⊙ Skipped provider configuration")
		selectedProvider = ""
	default:
		fmt.Println("\n✓ Using default: MAIA Router")
		selectedProvider = "maiarouter"
		defaultModel = "maia/gemini-3-pro-preview"
		providerURL = "https://maiarouter.ai"
	}

	// Step 2: API Key
	if selectedProvider != "" {
		fmt.Println("\nStep 2/5: API Key")
		fmt.Println("──────────────────────────────────")

		// Check for existing environment variable
		envKey, envVarName := config.GetProviderEnvKey(selectedProvider)
		envBase, envBaseName := config.GetProviderEnvBase(selectedProvider)

		var apiKey string
		var apiBase string

		if envKey != "" {
			// Mask the key for display (show first 8 chars)
			maskedKey := envKey
			if len(envKey) > 8 {
				maskedKey = envKey[:8] + "..." + envKey[len(envKey)-4:]
			}
			fmt.Printf("✓ Found API key in environment: %s=%s\n", envVarName, maskedKey)
			fmt.Print("Use this API key? (Y/n): ")
			useEnvKey, _ := reader.ReadString('\n')
			useEnvKey = strings.ToLower(strings.TrimSpace(useEnvKey))

			if useEnvKey != "n" && useEnvKey != "no" {
				apiKey = envKey
				fmt.Println("✓ Using environment variable for API key")
			} else {
				fmt.Print("Enter your API key: ")
				apiKey, _ = reader.ReadString('\n')
				apiKey = strings.TrimSpace(apiKey)
			}

			// Check for API base URL
			if envBase != "" {
				fmt.Printf("✓ Found API base in environment: %s=%s\n", envBaseName, envBase)
				fmt.Print("Use this API base? (Y/n): ")
				useEnvBase, _ := reader.ReadString('\n')
				useEnvBase = strings.ToLower(strings.TrimSpace(useEnvBase))

				if useEnvBase != "n" && useEnvBase != "no" {
					apiBase = envBase
				}
			}
		} else {
			fmt.Print("Enter your API key (or press Enter to skip): ")
			apiKey, _ = reader.ReadString('\n')
			apiKey = strings.TrimSpace(apiKey)
		}

		if apiKey != "" {
			switch selectedProvider {
			case "maiarouter":
				cfg.Providers.MAIARouter.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.MAIARouter.APIBase = apiBase
				} else {
					cfg.Providers.MAIARouter.APIBase = "https://api.maiarouter.ai/v1"
				}
			case "anthropic":
				cfg.Providers.Anthropic.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.Anthropic.APIBase = apiBase
				}
			case "openai":
				cfg.Providers.OpenAI.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.OpenAI.APIBase = apiBase
				}
			case "openrouter":
				cfg.Providers.OpenRouter.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.OpenRouter.APIBase = apiBase
				}
			case "gemini":
				cfg.Providers.Gemini.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.Gemini.APIBase = apiBase
				}
			case "groq":
				cfg.Providers.Groq.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.Groq.APIBase = apiBase
				}
			case "zhipu":
				cfg.Providers.Zhipu.APIKey = apiKey
				if apiBase != "" {
					cfg.Providers.Zhipu.APIBase = apiBase
				}
			}
			cfg.Agents.Defaults.Model = defaultModel
			fmt.Println("✓ API key configured")
		} else {
			fmt.Println("⊙ Skipped API key (you can add it later in config.json or environment)")
		}
	} else {
		fmt.Println("\nStep 2/5: API Key")
		fmt.Println("──────────────────────────────────")
		fmt.Println("⊙ Skipped (no provider selected)")
	}

	// Step 3: Channels
	fmt.Println("\nStep 3/5: Enable Chat Channels (optional)")
	fmt.Println("──────────────────────────────────")
	fmt.Println("Would you like to enable any chat channels?")
	fmt.Print("(T)elegram, (D)iscord, (W)hatsApp, or (N)one [T/D/W/N] (default: N): ")

	channelChoice, _ := reader.ReadString('\n')
	channelChoice = strings.ToLower(strings.TrimSpace(channelChoice))

	if channelChoice == "t" {
		// Check for existing Telegram token in environment
		envToken, envVarName := config.GetChannelEnvToken("telegram")
		var token string

		if envToken != "" {
			maskedToken := envToken
			if len(envToken) > 12 {
				maskedToken = envToken[:8] + "..." + envToken[len(envToken)-4:]
			}
			fmt.Printf("\n✓ Found Telegram token in environment: %s=%s\n", envVarName, maskedToken)
			fmt.Print("Use this token? (Y/n): ")
			useEnvToken, _ := reader.ReadString('\n')
			useEnvToken = strings.ToLower(strings.TrimSpace(useEnvToken))

			if useEnvToken != "n" && useEnvToken != "no" {
				token = envToken
				fmt.Println("✓ Using environment variable for token")
			} else {
				fmt.Print("Enter Telegram Bot Token: ")
				token, _ = reader.ReadString('\n')
				token = strings.TrimSpace(token)
			}
		} else {
			fmt.Print("\nTelegram Bot Token: ")
			token, _ = reader.ReadString('\n')
			token = strings.TrimSpace(token)
		}

		if token != "" {
			cfg.Channels.Telegram.Enabled = true
			cfg.Channels.Telegram.Token = token
			fmt.Println("✓ Telegram enabled")
			fmt.Println("  Note: Use /status in Telegram to check bot status")
		}
	} else if channelChoice == "d" {
		// Check for existing Discord token in environment
		envToken, envVarName := config.GetChannelEnvToken("discord")
		var token string

		if envToken != "" {
			maskedToken := envToken
			if len(envToken) > 12 {
				maskedToken = envToken[:8] + "..." + envToken[len(envToken)-4:]
			}
			fmt.Printf("\n✓ Found Discord token in environment: %s=%s\n", envVarName, maskedToken)
			fmt.Print("Use this token? (Y/n): ")
			useEnvToken, _ := reader.ReadString('\n')
			useEnvToken = strings.ToLower(strings.TrimSpace(useEnvToken))

			if useEnvToken != "n" && useEnvToken != "no" {
				token = envToken
				fmt.Println("✓ Using environment variable for token")
			} else {
				fmt.Print("Enter Discord Bot Token: ")
				token, _ = reader.ReadString('\n')
				token = strings.TrimSpace(token)
			}
		} else {
			fmt.Print("\nDiscord Bot Token: ")
			token, _ = reader.ReadString('\n')
			token = strings.TrimSpace(token)
		}

		if token != "" {
			cfg.Channels.Discord.Enabled = true
			cfg.Channels.Discord.Token = token
			fmt.Println("✓ Discord enabled")
		}
	} else if channelChoice == "w" {
		cfg.Channels.WhatsApp.Enabled = true
		fmt.Println("\n✓ WhatsApp enabled (scan QR code when gateway starts)")
	} else {
		fmt.Println("⊙ No channels enabled (you can enable them later)")
	}

	// Step 4: Workspace
	fmt.Println("\nStep 4/5: Workspace Setup")
	fmt.Println("──────────────────────────────────")
	fmt.Printf("Default workspace: %s\n", cfg.WorkspacePath())
	fmt.Print("Use default? (Y/n): ")

	workspaceChoice, _ := reader.ReadString('\n')
	workspaceChoice = strings.ToLower(strings.TrimSpace(workspaceChoice))

	if workspaceChoice == "n" {
		fmt.Print("Enter workspace path: ")
		customWorkspace, _ := reader.ReadString('\n')
		customWorkspace = strings.TrimSpace(customWorkspace)
		if customWorkspace != "" {
			cfg.Agents.Defaults.Workspace = customWorkspace
		}
	}

	// Save configuration
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Saving configuration...")

	if err := config.SaveConfig(configPath, cfg); err != nil {
		fmt.Printf("✗ Error saving config: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Config saved to: %s\n", configPath)

	// Create workspace structure
	workspace := cfg.WorkspacePath()
	os.MkdirAll(workspace, 0755)
	os.MkdirAll(filepath.Join(workspace, "memory"), 0755)
	os.MkdirAll(filepath.Join(workspace, "skills"), 0755)
	fmt.Printf("✓ Workspace created at: %s\n", workspace)

	// Create workspace templates
	createWorkspaceTemplates(workspace)

	// Step 5: Install builtin skills
	fmt.Println("\nStep 5/5: Install Builtin Skills")
	fmt.Println("──────────────────────────────────")
	fmt.Println("This will download skills from: https://github.com/pepebot-space/skills-builtin")
	fmt.Print("Install builtin skills? (Y/n): ")

	builtinChoice, _ := reader.ReadString('\n')
	builtinChoice = strings.ToLower(strings.TrimSpace(builtinChoice))

	builtinInstalled := false
	if builtinChoice != "n" && builtinChoice != "no" {
		fmt.Println("")
		installer := skills.NewSkillInstaller(workspace)
		skillsInstallBuiltinCmd(installer)
		builtinInstalled = true
	} else {
		fmt.Println("⊙ Skipped builtin skills installation")
		fmt.Println("  You can install them later with: pepebot skills install-builtin")
	}

	// Success message
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("     ___")
	fmt.Println("    (^ ^)")
	fmt.Println("   (  v  )   🎉 SETUP COMPLETE!")
	fmt.Println("   /|   |\\")
	fmt.Println("  (_|   |_)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Println("\n📚 Quick Start:")
	fmt.Println("  • Test in CLI:     pepebot agent -m \"Hello!\"")
	fmt.Println("  • Interactive:     pepebot agent")
	if cfg.Channels.Telegram.Enabled || cfg.Channels.Discord.Enabled || cfg.Channels.WhatsApp.Enabled {
		fmt.Println("  • Start gateway:   pepebot gateway")
	}
	fmt.Println("  • View status:     pepebot status")
	if !builtinInstalled {
		fmt.Println("  • Install skills:  pepebot skills install-builtin")
	}

	if selectedProvider != "" && (selectedProvider == "maiarouter" || selectedProvider == "anthropic" || selectedProvider == "openai") {
		fmt.Println("\n💡 Tips:")
		if selectedProvider == "maiarouter" {
			fmt.Println("  • MAIA Router offers 52+ free models!")
			fmt.Println("  • Try: maia/gemini-2.5-flash (fast & free)")
			fmt.Println("  • Payment: QRIS supported for Indonesian users")
		}
		fmt.Printf("  • Edit config: %s\n", configPath)
		fmt.Printf("  • Documentation: https://github.com/pepebot-space/pepebot\n")
	}

	fmt.Println("\n🎉 Happy chatting with Pepebot!")
}

func createWorkspaceTemplates(workspace string) {
	templates := map[string]string{
		"AGENTS.md": `# Agent Instructions

You are a helpful AI assistant. Be concise, accurate, and friendly.

## Guidelines

- Always explain what you're doing before taking actions
- Ask for clarification when request is ambiguous
- Use tools to help accomplish tasks
- Remember important information in your memory files
- Be proactive and helpful
- Learn from user feedback
`,
		"SOUL.md": `# Soul

I am pepebot, a lightweight AI assistant powered by AI.

## Personality

- Helpful and friendly
- Concise and to the point
- Curious and eager to learn
- Honest and transparent

## Values

- Accuracy over speed
- User privacy and safety
- Transparency in actions
- Continuous improvement
`,
		"USER.md": `# User

Information about user goes here.

## Preferences

- Communication style: (casual/formal)
- Timezone: (your timezone)
- Language: (your preferred language)

## Personal Information

- Name: (optional)
- Location: (optional)
- Occupation: (optional)

## Learning Goals

- What the user wants to learn from AI
- Preferred interaction style
- Areas of interest
`,
		"TOOLS.md": `# Available Tools

This document describes the tools available to pepebot.

## File Operations

### Read Files
- Read file contents
- Supports text, markdown, code files

### Write Files
- Create new files
- Overwrite existing files
- Supports various formats

### List Directories
- List directory contents
- Recursive listing support

### Edit Files
- Make specific edits to files
- Line-by-line editing
- String replacement

## Web Tools

### Web Search
- Search the internet using search API
- Returns titles, URLs, snippets
- Optional: Requires API key for best results

### Web Fetch
- Fetch specific URLs
- Extract readable content
- Supports HTML, JSON, plain text
- Automatic content extraction

## Command Execution

### Shell Commands
- Execute any shell command
- Run in workspace directory
- Full shell access with timeout protection

## Messaging

### Send Messages
- Send messages to chat channels
- Supports Telegram, WhatsApp, Feishu
- Used for notifications and responses

## Android Device Control (ADB)

### ADB Tools
- adb_devices: List connected Android devices
- adb_shell: Execute shell commands on device
- adb_tap: Tap screen coordinates
- adb_swipe: Swipe gestures on screen
- adb_input_text: Input text into focused field
- adb_screenshot: Capture device screenshot
- adb_ui_dump: Get UI hierarchy XML
- adb_open_app: Launch app by package name
- adb_keyevent: Send key events (Home, Back, etc.)

### ADB Activity Recorder
- adb_record_workflow: Record user interactions (taps, swipes) from Android device and auto-generate a workflow file
- Use this when user says "workflow action", "record workflow", "capture actions", "rekam aksi", etc.
- This captures real device interactions - do NOT use workflow_save for this purpose
- Flow: explain to user → get confirmation → start recording → user interacts with device → Volume Down to stop → workflow saved

## Workflow System

### Workflow Tools (Agent)
- workflow_execute: Run a saved workflow with optional variable overrides
- workflow_save: Manually create a workflow JSON (for when YOU write the steps)
- workflow_list: List available workflows

### Workflow CLI (Standalone)
Users can also run workflows directly from the terminal without the agent:

` + "`" + `pepebot workflow list` + "`" + `                        — List all workflows
` + "`" + `pepebot workflow show <name>` + "`" + `               — Show workflow details
` + "`" + `pepebot workflow run <name>` + "`" + `                — Execute a workflow from workspace
` + "`" + `pepebot workflow run <name> --var k=v` + "`" + `     — Execute with variable overrides
` + "`" + `pepebot workflow run -f /path/to/file.json` + "`" + ` — Execute directly from any JSON file
` + "`" + `pepebot workflow validate <name>` + "`" + `            — Validate workflow structure
` + "`" + `pepebot workflow delete <name>` + "`" + `              — Delete a workflow

This enables cron scheduling, shell scripts, CI/CD pipelines, and any automation that chains workflows without needing the agent.

## AI Capabilities

### Context Building
- Load system instructions from files
- Load skills dynamically
- Build conversation history
- Include timezone and other context

### Memory Management
- Long-term memory via MEMORY.md
- Daily notes via dated files
- Persistent across sessions
`,
		"IDENTITY.md": `# Identity

## Name
Pepebot 🐸

## Description
Ultra-lightweight personal AI assistant.

## Version
0.1.0

## Purpose
- Provide intelligent AI assistance with minimal resource usage
- Support multiple LLM providers (OpenAI, Anthropic, Zhipu, etc.)
- Enable easy customization through skills system
- Run on minimal hardware ($10 boards, <10MB RAM)

## Capabilities

- Web search and content fetching
- File system operations (read, write, edit)
- Shell command execution
- Multi-channel messaging (Telegram, WhatsApp, Feishu)
- Skill-based extensibility
- Memory and context management

## Philosophy

- Simplicity over complexity
- Performance over features
- User control and privacy
- Transparent operation
- Community-driven development

## Goals

- Provide a fast, lightweight AI assistant
- Support offline-first operation where possible
- Enable easy customization and extension
- Maintain high quality responses
- Run efficiently on constrained hardware

## License
MIT License - Free and open source

## Repository
https://github.com/sipeed/pepebot

## Contact
Issues: https://github.com/sipeed/pepebot/issues
Discussions: https://github.com/sipeed/pepebot/discussions

---

"Every bit helps, every bit matters."
- Pepebot
`,
	}

	for filename, content := range templates {
		filePath := filepath.Join(workspace, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			os.WriteFile(filePath, []byte(content), 0644)
			fmt.Printf("  Created %s\n", filename)
		}
	}

	memoryDir := filepath.Join(workspace, "memory")
	os.MkdirAll(memoryDir, 0755)
	memoryFile := filepath.Join(memoryDir, "MEMORY.md")
	if _, err := os.Stat(memoryFile); os.IsNotExist(err) {
		memoryContent := `# Long-term Memory

This file stores important information that should persist across sessions.

## User Information

(Important facts about user)

## Preferences

(User preferences learned over time)

## Important Notes

(Things to remember)

## Configuration

- Model preferences
- Channel settings
- Skills enabled
`
		os.WriteFile(memoryFile, []byte(memoryContent), 0644)
		fmt.Println("  Created memory/MEMORY.md")

		skillsDir := filepath.Join(workspace, "skills")
		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			os.MkdirAll(skillsDir, 0755)
			fmt.Println("  Created skills/")
		}
	}

	for filename, content := range templates {
		filePath := filepath.Join(workspace, filename)
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			os.WriteFile(filePath, []byte(content), 0644)
			fmt.Printf("  Created %s\n", filename)
		}
	}
}

func agentCmd() {
	message := ""
	sessionKey := "cli:default"
	agentName := "" // empty = use default agent

	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-m", "--message":
			if i+1 < len(args) {
				message = args[i+1]
				i++
			}
		case "-s", "--session":
			if i+1 < len(args) {
				sessionKey = args[i+1]
				i++
			}
		case "-a", "--agent":
			if i+1 < len(args) {
				agentName = args[i+1]
				i++
			}
		}
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	provider, err := providers.CreateProvider(cfg)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		os.Exit(1)
	}

	bus := bus.NewMessageBus()

	// Create agent manager for multi-agent support
	agentManager, err := agent.NewAgentManager(cfg, bus, provider)
	if err != nil {
		fmt.Printf("Error creating agent manager: %v\n", err)
		os.Exit(1)
	}

	// Get the specific agent or default agent
	var agentLoop *agent.AgentLoop
	if agentName != "" {
		agentLoop, err = agentManager.GetOrCreateAgent(agentName)
		if err != nil {
			fmt.Printf("Error getting agent '%s': %v\n", agentName, err)
			os.Exit(1)
		}
		fmt.Printf("Using agent: %s\n", agentName)
	} else {
		agentLoop, err = agentManager.GetDefaultAgent()
		if err != nil {
			fmt.Printf("Error getting default agent: %v\n", err)
			os.Exit(1)
		}
	}

	if message != "" {
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, message, nil, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n%s %s\n", logo, response)
	} else {
		fmt.Printf("%s Interactive mode (Ctrl+C to exit)\n\n", logo)
		interactiveMode(agentLoop, sessionKey)
	}
}

func handleCLICommand(input string, agentLoop *agent.AgentLoop, sessionKey string) bool {
	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	switch command {
	case "/new":
		agentLoop.ClearSession(sessionKey)
		fmt.Printf("\n%s Session cleared. Starting fresh conversation.\n\n", logo)
		return true
	case "/help":
		fmt.Printf("\n%s Available commands:\n", logo)
		fmt.Println("  /new    - Clear session, start fresh conversation")
		fmt.Println("  /help   - Show this help message")
		fmt.Println("  /status - Show agent & session info")
		fmt.Println("  exit    - Exit interactive mode")
		fmt.Println()
		return true
	case "/status":
		fmt.Printf("\n%s Agent: %s\n", logo, agentLoop.AgentName())
		fmt.Printf("  Model: %s\n", agentLoop.Model())
		fmt.Printf("  Session: %s\n\n", sessionKey)
		return true
	}

	return false
}

func interactiveMode(agentLoop *agent.AgentLoop, sessionKey string) {
	prompt := fmt.Sprintf("%s You: ", logo)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     "/tmp/.pepebot_history",
		HistoryLimit:    100,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})

	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		fmt.Println("Falling back to simple input mode...")
		simpleInteractiveMode(agentLoop, sessionKey)
		return
	}
	defer rl.Close()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		if strings.HasPrefix(input, "/") {
			if handleCLICommand(input, agentLoop, sessionKey) {
				continue
			}
		}

		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, nil, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", logo, response)
	}
}

func simpleInteractiveMode(agentLoop *agent.AgentLoop, sessionKey string) {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print(fmt.Sprintf("%s You: ", logo))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		if strings.HasPrefix(input, "/") {
			if handleCLICommand(input, agentLoop, sessionKey) {
				continue
			}
		}

		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, nil, sessionKey)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Printf("\n%s %s\n\n", logo, response)
	}
}

func gatewayCmd() {
	// Parse flags
	verbose := false
	args := os.Args[2:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-v", "--verbose":
			verbose = true
		}
	}

	// Enable verbose logging if requested
	if verbose {
		logger.SetLevel(logger.DEBUG)
		fmt.Println("✓ Verbose logging enabled")
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	notifyRestartSignal(sigChan)

	for {
		shouldRestart := gatewayRun(sigChan)
		if !shouldRestart {
			break
		}
		fmt.Println("\n🔄 Restarting gateway...")
		// Small delay to let connections drain
		time.Sleep(500 * time.Millisecond)
	}
}

// gatewayRun starts all gateway services and blocks until a signal is received.
// Returns true if a restart was requested (SIGHUP), false if shutdown (SIGINT).
func gatewayRun(sigChan chan os.Signal) bool {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	provider, err := providers.CreateProvider(cfg)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		os.Exit(1)
	}

	msgBus := bus.NewMessageBus()

	// Create agent manager for multi-agent support
	agentManager, err := agent.NewAgentManager(cfg, msgBus, provider)
	if err != nil {
		fmt.Printf("Error creating agent manager: %v\n", err)
		os.Exit(1)
	}

	// List enabled agents
	enabledAgents := agentManager.ListEnabledAgents()
	if len(enabledAgents) > 0 {
		agentNames := make([]string, 0, len(enabledAgents))
		for name := range enabledAgents {
			agentNames = append(agentNames, name)
		}
		fmt.Printf("✓ Agents enabled: %v\n", agentNames)
	}

	cronStorePath := filepath.Join(filepath.Dir(getConfigPath()), "cron", "jobs.json")
	cronService := cron.NewCronService(cronStorePath, nil)

	heartbeatService := heartbeat.NewHeartbeatService(
		cfg.WorkspacePath(),
		nil,
		30*60,
		true,
	)

	channelManager, err := channels.NewManager(cfg, msgBus)
	if err != nil {
		fmt.Printf("Error creating channel manager: %v\n", err)
		os.Exit(1)
	}

	var transcriber *voice.GroqTranscriber
	if cfg.Providers.Groq.APIKey != "" {
		transcriber = voice.NewGroqTranscriber(cfg.Providers.Groq.APIKey)
		logger.InfoC("voice", "Groq voice transcription enabled")
	}

	if transcriber != nil {
		if telegramChannel, ok := channelManager.GetChannel("telegram"); ok {
			if tc, ok := telegramChannel.(*channels.TelegramChannel); ok {
				tc.SetTranscriber(transcriber)
				logger.InfoC("voice", "Groq transcription attached to Telegram channel")
			}
		}
	}

	enabledChannels := channelManager.GetEnabledChannels()
	if len(enabledChannels) > 0 {
		fmt.Printf("✓ Channels enabled: %s\n", enabledChannels)
	} else {
		fmt.Println("⚠ Warning: No channels enabled")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Restart function: sends SIGHUP to self to trigger graceful restart
	restartFunc := func() {
		triggerRestart()
	}

	// Start HTTP API server (with restart support)
	gatewayServer := gateway.NewGatewayServer(cfg, agentManager, msgBus)
	gatewayServer.SetRestartFunc(restartFunc)
	agentManager.SetRestartFunc(restartFunc)
	if err := gatewayServer.Start(ctx); err != nil {
		fmt.Printf("Error starting HTTP API server: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ HTTP API server started on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)

	if err := cronService.Start(); err != nil {
		fmt.Printf("Error starting cron service: %v\n", err)
	}
	fmt.Println("✓ Cron service started")

	if err := heartbeatService.Start(); err != nil {
		fmt.Printf("Error starting heartbeat service: %v\n", err)
	}
	fmt.Println("✓ Heartbeat service started")

	if err := channelManager.StartAll(ctx); err != nil {
		fmt.Printf("Error starting channels: %v\n", err)
	}

	go agentManager.Run(ctx)

	fmt.Printf("✓ Gateway started on %s:%d\n", cfg.Gateway.Host, cfg.Gateway.Port)
	fmt.Println("Press Ctrl+C to stop")

	sig := <-sigChan

	restart := isRestartSignal(sig)

	if restart {
		fmt.Println("\nRestarting...")
	} else {
		fmt.Println("\nShutting down...")
	}
	cancel()
	gatewayServer.Stop(context.Background())
	heartbeatService.Stop()
	cronService.Stop()
	channelManager.StopAll(context.Background())

	if restart {
		fmt.Println("✓ Gateway stopped (restarting)")
	} else {
		fmt.Println("✓ Gateway stopped")
	}

	return restart
}

func statusCmd() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	configPath := getConfigPath()

	fmt.Printf("%s pepebot Status\n\n", logo)

	if _, err := os.Stat(configPath); err == nil {
		fmt.Println("Config:", configPath, "✓")
	} else {
		fmt.Println("Config:", configPath, "✗")
	}

	workspace := cfg.WorkspacePath()
	if _, err := os.Stat(workspace); err == nil {
		fmt.Println("Workspace:", workspace, "✓")
	} else {
		fmt.Println("Workspace:", workspace, "✗")
	}

	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("Model: %s\n", cfg.Agents.Defaults.Model)

		hasMAIARouter := cfg.Providers.MAIARouter.APIKey != ""
		hasOpenRouter := cfg.Providers.OpenRouter.APIKey != ""
		hasAnthropic := cfg.Providers.Anthropic.APIKey != ""
		hasOpenAI := cfg.Providers.OpenAI.APIKey != ""
		hasGemini := cfg.Providers.Gemini.APIKey != ""
		hasZhipu := cfg.Providers.Zhipu.APIKey != ""
		hasGroq := cfg.Providers.Groq.APIKey != ""
		hasVLLM := cfg.Providers.VLLM.APIBase != ""

		status := func(enabled bool) string {
			if enabled {
				return "✓"
			}
			return "not set"
		}
		fmt.Println("\nProviders:")
		fmt.Println("MAIA Router:", status(hasMAIARouter))
		fmt.Println("OpenRouter API:", status(hasOpenRouter))
		fmt.Println("Anthropic API:", status(hasAnthropic))
		fmt.Println("OpenAI API:", status(hasOpenAI))
		fmt.Println("Gemini API:", status(hasGemini))
		fmt.Println("Zhipu API:", status(hasZhipu))
		fmt.Println("Groq API:", status(hasGroq))
		if hasVLLM {
			fmt.Printf("vLLM/Local: ✓ %s\n", cfg.Providers.VLLM.APIBase)
		} else {
			fmt.Println("vLLM/Local: not set")
		}
	}
}

func getConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pepebot", "config.json")
}

func loadConfig() (*config.Config, error) {
	return config.LoadConfig(getConfigPath())
}

func cronCmd() {
	if len(os.Args) < 3 {
		cronHelp()
		return
	}

	subcommand := os.Args[2]

	dataDir := filepath.Join(filepath.Dir(getConfigPath()), "cron")
	cronStorePath := filepath.Join(dataDir, "jobs.json")

	switch subcommand {
	case "list":
		cronListCmd(cronStorePath)
	case "add":
		cronAddCmd(cronStorePath)
	case "remove":
		if len(os.Args) < 4 {
			fmt.Println("Usage: pepebot cron remove <job_id>")
			return
		}
		cronRemoveCmd(cronStorePath, os.Args[3])
	case "enable":
		cronEnableCmd(cronStorePath, false)
	case "disable":
		cronEnableCmd(cronStorePath, true)
	default:
		fmt.Printf("Unknown cron command: %s\n", subcommand)
		cronHelp()
	}
}

func cronHelp() {
	fmt.Println("\nCron commands:")
	fmt.Println("  list              List all scheduled jobs")
	fmt.Println("  add              Add a new scheduled job")
	fmt.Println("  remove <id>       Remove a job by ID")
	fmt.Println("  enable <id>      Enable a job")
	fmt.Println("  disable <id>     Disable a job")
	fmt.Println()
	fmt.Println("Add options:")
	fmt.Println("  -n, --name       Job name")
	fmt.Println("  -m, --message    Message for agent")
	fmt.Println("  -e, --every      Run every N seconds")
	fmt.Println("  -c, --cron       Cron expression (e.g. '0 9 * * *')")
	fmt.Println("  -d, --deliver     Deliver response to channel")
	fmt.Println("  --to             Recipient for delivery")
	fmt.Println("  --channel        Channel for delivery")
}

func cronListCmd(storePath string) {
	cs := cron.NewCronService(storePath, nil)
	jobs := cs.ListJobs(false)

	if len(jobs) == 0 {
		fmt.Println("No scheduled jobs.")
		return
	}

	fmt.Println("\nScheduled Jobs:")
	fmt.Println("----------------")
	for _, job := range jobs {
		var schedule string
		if job.Schedule.Kind == "every" && job.Schedule.EveryMS != nil {
			schedule = fmt.Sprintf("every %ds", *job.Schedule.EveryMS/1000)
		} else if job.Schedule.Kind == "cron" {
			schedule = job.Schedule.Expr
		} else {
			schedule = "one-time"
		}

		nextRun := "scheduled"
		if job.State.NextRunAtMS != nil {
			nextTime := time.UnixMilli(*job.State.NextRunAtMS)
			nextRun = nextTime.Format("2006-01-02 15:04")
		}

		status := "enabled"
		if !job.Enabled {
			status = "disabled"
		}

		fmt.Printf("  %s (%s)\n", job.Name, job.ID)
		fmt.Printf("    Schedule: %s\n", schedule)
		fmt.Printf("    Status: %s\n", status)
		fmt.Printf("    Next run: %s\n", nextRun)
	}
}

func cronAddCmd(storePath string) {
	name := ""
	message := ""
	var everySec *int64
	cronExpr := ""
	deliver := false
	channel := ""
	to := ""

	args := os.Args[3:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n", "--name":
			if i+1 < len(args) {
				name = args[i+1]
				i++
			}
		case "-m", "--message":
			if i+1 < len(args) {
				message = args[i+1]
				i++
			}
		case "-e", "--every":
			if i+1 < len(args) {
				var sec int64
				fmt.Sscanf(args[i+1], "%d", &sec)
				everySec = &sec
				i++
			}
		case "-c", "--cron":
			if i+1 < len(args) {
				cronExpr = args[i+1]
				i++
			}
		case "-d", "--deliver":
			deliver = true
		case "--to":
			if i+1 < len(args) {
				to = args[i+1]
				i++
			}
		case "--channel":
			if i+1 < len(args) {
				channel = args[i+1]
				i++
			}
		}
	}

	if name == "" {
		fmt.Println("Error: --name is required")
		return
	}

	if message == "" {
		fmt.Println("Error: --message is required")
		return
	}

	if everySec == nil && cronExpr == "" {
		fmt.Println("Error: Either --every or --cron must be specified")
		return
	}

	var schedule cron.CronSchedule
	if everySec != nil {
		everyMS := *everySec * 1000
		schedule = cron.CronSchedule{
			Kind:    "every",
			EveryMS: &everyMS,
		}
	} else {
		schedule = cron.CronSchedule{
			Kind: "cron",
			Expr: cronExpr,
		}
	}

	cs := cron.NewCronService(storePath, nil)
	job, err := cs.AddJob(name, schedule, message, deliver, channel, to)
	if err != nil {
		fmt.Printf("Error adding job: %v\n", err)
		return
	}

	fmt.Printf("✓ Added job '%s' (%s)\n", job.Name, job.ID)
}

func cronRemoveCmd(storePath, jobID string) {
	cs := cron.NewCronService(storePath, nil)
	if cs.RemoveJob(jobID) {
		fmt.Printf("✓ Removed job %s\n", jobID)
	} else {
		fmt.Printf("✗ Job %s not found\n", jobID)
	}
}

func cronEnableCmd(storePath string, disable bool) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot cron enable/disable <job_id>")
		return
	}

	jobID := os.Args[3]
	cs := cron.NewCronService(storePath, nil)
	enabled := !disable

	job := cs.EnableJob(jobID, enabled)
	if job != nil {
		status := "enabled"
		if disable {
			status = "disabled"
		}
		fmt.Printf("✓ Job '%s' %s\n", job.Name, status)
	} else {
		fmt.Printf("✗ Job %s not found\n", jobID)
	}
}

func skillsCmd() {
	if len(os.Args) < 3 {
		skillsHelp()
		return
	}

	subcommand := os.Args[2]

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	workspace := cfg.WorkspacePath()
	installer := skills.NewSkillInstaller(workspace)
	skillsLoader := skills.NewSkillsLoader(workspace, "")

	switch subcommand {
	case "list":
		skillsListCmd(skillsLoader)
	case "install":
		skillsInstallCmd(installer)
	case "remove", "uninstall":
		if len(os.Args) < 4 {
			fmt.Println("Usage: pepebot skills remove <skill-name>")
			return
		}
		skillsRemoveCmd(installer, os.Args[3])
	case "search":
		skillsSearchCmd(installer)
	case "show":
		if len(os.Args) < 4 {
			fmt.Println("Usage: pepebot skills show <skill-name>")
			return
		}
		skillsShowCmd(skillsLoader, os.Args[3])
	default:
		fmt.Printf("Unknown skills command: %s\n", subcommand)
		skillsHelp()
	}
}

func skillsHelp() {
	fmt.Println("\nSkills commands:")
	fmt.Println("  list                    List installed skills")
	fmt.Println("  install <repo>          Install skill from GitHub")
	fmt.Println("  install-builtin         Install all builtin skills from pepebot-space/skills-builtin")
	fmt.Println("  remove <name>           Remove installed skill")
	fmt.Println("  search                  Search available skills")
	fmt.Println("  show <name>             Show skill details")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  pepebot skills list")
	fmt.Println("  pepebot skills install pepebot/skills/weather")
	fmt.Println("  pepebot skills install-builtin")
	fmt.Println("  pepebot skills remove weather")
}

func skillsListCmd(loader *skills.SkillsLoader) {
	allSkills := loader.ListSkills(false)

	if len(allSkills) == 0 {
		fmt.Println("No skills installed.")
		return
	}

	fmt.Println("\nInstalled Skills:")
	fmt.Println("------------------")
	for _, skill := range allSkills {
		status := "✓"
		if !skill.Available {
			status = "✗"
		}
		fmt.Printf("  %s %s (%s)\n", status, skill.Name, skill.Source)
		if skill.Description != "" {
			fmt.Printf("    %s\n", skill.Description)
		}
		if !skill.Available {
			fmt.Printf("    Missing: %s\n", skill.Missing)
		}
	}
}

func skillsInstallCmd(installer *skills.SkillInstaller) {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot skills install <github-repo>")
		fmt.Println("Example: pepebot skills install pepebot/skills/weather")
		return
	}

	repo := os.Args[3]
	fmt.Printf("Installing skill from %s...\n", repo)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := installer.InstallFromGitHub(ctx, repo); err != nil {
		fmt.Printf("✗ Failed to install skill: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Skill '%s' installed successfully!\n", filepath.Base(repo))
}

func skillsRemoveCmd(installer *skills.SkillInstaller, skillName string) {
	fmt.Printf("Removing skill '%s'...\n", skillName)

	if err := installer.Uninstall(skillName); err != nil {
		fmt.Printf("✗ Failed to remove skill: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Skill '%s' removed successfully!\n", skillName)
}

func skillsInstallBuiltinCmd(installer *skills.SkillInstaller) {
	fmt.Println("Installing builtin skills from https://github.com/pepebot-space/skills-builtin")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := installer.InstallBuiltinSkills(ctx); err != nil {
		fmt.Printf("✗ Failed to install builtin skills: %v\n", err)
		if strings.Contains(err.Error(), "HTTP 404") {
			fmt.Println("\n  ℹ Note: The builtin skills repository might not be available yet.")
			fmt.Println("  Please check: https://github.com/pepebot-space/skills-builtin")
			fmt.Println("  Or install skills manually from other sources:")
			fmt.Println("    pepebot skills install <github-repo>")
		} else {
			fmt.Println("  You can try again later with: pepebot skills install-builtin")
		}
		return
	}

	fmt.Println("\n✓ Builtin skills installed successfully!")
	fmt.Println("  Use 'pepebot skills list' to see installed skills")
}

func skillsSearchCmd(installer *skills.SkillInstaller) {
	fmt.Println("Searching for available skills...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	availableSkills, err := installer.ListAvailableSkills(ctx)
	if err != nil {
		fmt.Printf("✗ Failed to fetch skills list: %v\n", err)
		return
	}

	if len(availableSkills) == 0 {
		fmt.Println("No skills available.")
		return
	}

	fmt.Printf("\nAvailable Skills (%d):\n", len(availableSkills))
	fmt.Println("--------------------")
	for _, skill := range availableSkills {
		fmt.Printf("  📦 %s\n", skill.Name)
		fmt.Printf("     %s\n", skill.Description)
		fmt.Printf("     Install: pepebot skills install pepebot-space/skills/%s\n", skill.Path)
		fmt.Println()
	}
}

func skillsShowCmd(loader *skills.SkillsLoader, skillName string) {
	content, ok := loader.LoadSkill(skillName)
	if !ok {
		fmt.Printf("✗ Skill '%s' not found\n", skillName)
		return
	}

	fmt.Printf("\n📦 Skill: %s\n", skillName)
	fmt.Println("----------------------")
	fmt.Println(content)
}

// =============================================================================
// Agent Management Commands
// =============================================================================

func loadAgentRegistry() (*agent.AgentRegistry, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	registry := agent.NewAgentRegistry(cfg.WorkspacePath())

	// Load existing registry
	if err := registry.Load(); err != nil {
		return nil, fmt.Errorf("failed to load registry: %w", err)
	}

	// Initialize from config if empty
	if err := registry.InitializeFromConfig(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize registry: %w", err)
	}

	return registry, nil
}

func agentHelpCmd() {
	fmt.Println("\n🐸 Pepebot Agent Management")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Println("Usage: pepebot agent <subcommand> [options]\n")
	fmt.Println("Subcommands:")
	fmt.Println("  list                    List all registered agents")
	fmt.Println("  register <name>         Register a new agent")
	fmt.Println("  remove <name>           Remove an agent")
	fmt.Println("  enable <name>           Enable an agent")
	fmt.Println("  disable <name>          Disable an agent")
	fmt.Println("  show <name>             Show agent details")
	fmt.Println("  help                    Show this help message")
	fmt.Println("\nOptions for 'register':")
	fmt.Println("  --model <model>         Model to use (required)")
	fmt.Println("  --provider <provider>   Provider name (optional)")
	fmt.Println("  --description <desc>    Agent description (optional)")
	fmt.Println("  --temperature <temp>    Temperature (0.0-1.0, optional)")
	fmt.Println("  --max-tokens <n>        Max tokens (optional)")
	fmt.Println("\nExamples:")
	fmt.Println("  pepebot agent list")
	fmt.Println("  pepebot agent register coder --model \"maia/claude-3-5-sonnet\" --description \"Coding specialist\"")
	fmt.Println("  pepebot agent enable coder")
	fmt.Println("  pepebot agent show coder")
	fmt.Println("  pepebot agent remove coder")
	fmt.Println()
}

func agentListCmd() {
	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	agents := registry.List()
	if len(agents) == 0 {
		fmt.Println("No agents registered")
		return
	}

	fmt.Println("\n🐸 Registered Agents")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// Sort agent names for consistent output
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}

	for _, name := range names {
		agent := agents[name]
		status := "✗ disabled"
		if agent.Enabled {
			status = "✓ enabled"
		}

		fmt.Printf("  %s  %s\n", status, name)
		fmt.Printf("           Model: %s\n", agent.Model)
		if agent.Description != "" {
			fmt.Printf("           Description: %s\n", agent.Description)
		}
		if agent.Temperature > 0 {
			fmt.Printf("           Temperature: %.1f\n", agent.Temperature)
		}
		if agent.MaxTokens > 0 {
			fmt.Printf("           Max Tokens: %d\n", agent.MaxTokens)
		}
		fmt.Println()
	}
}

func agentRegisterCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot agent register <name> --model <model> [options]")
		fmt.Println("Run 'pepebot agent help' for more information")
		os.Exit(1)
	}

	name := os.Args[3]
	var model, provider, description string
	var temperature float64
	var maxTokens int

	// Parse flags
	args := os.Args[4:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--provider":
			if i+1 < len(args) {
				provider = args[i+1]
				i++
			}
		case "--description":
			if i+1 < len(args) {
				description = args[i+1]
				i++
			}
		case "--temperature":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%f", &temperature)
				i++
			}
		case "--max-tokens":
			if i+1 < len(args) {
				fmt.Sscanf(args[i+1], "%d", &maxTokens)
				i++
			}
		}
	}

	if model == "" {
		fmt.Println("Error: --model is required")
		os.Exit(1)
	}

	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	agentDef := &agent.AgentDefinition{
		Enabled:     true,
		Model:       model,
		Provider:    provider,
		Description: description,
		Temperature: temperature,
		MaxTokens:   maxTokens,
	}

	if err := registry.Register(name, agentDef); err != nil {
		fmt.Printf("Error registering agent: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Save(); err != nil {
		fmt.Printf("Error saving registry: %v\n", err)
		os.Exit(1)
	}

	// Create agent-specific directory for bootstrap files
	agentDir, err := registry.EnsureAgentDir(name)
	if err != nil {
		fmt.Printf("Warning: could not create agent directory: %v\n", err)
	}

	fmt.Printf("✓ Registered agent '%s' with model '%s'\n", name, model)
	if agentDir != "" {
		fmt.Printf("  Agent directory: %s\n", agentDir)
		fmt.Printf("  Tip: Add SOUL.md, USER.md, etc. in this folder to personalize the agent\n")
	}
}

func agentRemoveCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot agent remove <name>")
		os.Exit(1)
	}

	name := os.Args[3]

	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Unregister(name); err != nil {
		fmt.Printf("Error removing agent: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Save(); err != nil {
		fmt.Printf("Error saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Removed agent '%s'\n", name)
}

func agentEnableCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot agent enable <name>")
		os.Exit(1)
	}

	name := os.Args[3]

	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Enable(name); err != nil {
		fmt.Printf("Error enabling agent: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Save(); err != nil {
		fmt.Printf("Error saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Enabled agent '%s'\n", name)
}

func agentDisableCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot agent disable <name>")
		os.Exit(1)
	}

	name := os.Args[3]

	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Disable(name); err != nil {
		fmt.Printf("Error disabling agent: %v\n", err)
		os.Exit(1)
	}

	if err := registry.Save(); err != nil {
		fmt.Printf("Error saving registry: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Disabled agent '%s'\n", name)
}

func agentShowCmd() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: pepebot agent show <name>")
		os.Exit(1)
	}

	name := os.Args[3]

	registry, err := loadAgentRegistry()
	if err != nil {
		fmt.Printf("Error loading registry: %v\n", err)
		os.Exit(1)
	}

	agentDef, err := registry.Get(name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	status := "disabled"
	if agentDef.Enabled {
		status = "enabled"
	}

	fmt.Printf("\n🐸 Agent: %s\n", name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("  Status:      %s\n", status)
	fmt.Printf("  Model:       %s\n", agentDef.Model)
	if agentDef.Provider != "" {
		fmt.Printf("  Provider:    %s\n", agentDef.Provider)
	}
	if agentDef.Description != "" {
		fmt.Printf("  Description: %s\n", agentDef.Description)
	}
	if agentDef.Temperature > 0 {
		fmt.Printf("  Temperature: %.1f\n", agentDef.Temperature)
	}
	if agentDef.MaxTokens > 0 {
		fmt.Printf("  Max Tokens:  %d\n", agentDef.MaxTokens)
	}
	if agentDef.PromptFile != "" {
		fmt.Printf("  Prompt Dir:  %s\n", agentDef.PromptFile)
		// Check if directory exists and list files
		if entries, err := os.ReadDir(agentDef.PromptFile); err == nil {
			if len(entries) > 0 {
				fmt.Printf("  Bootstrap files:\n")
				for _, entry := range entries {
					if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
						fmt.Printf("    - %s\n", entry.Name())
					}
				}
			} else {
				fmt.Printf("  Bootstrap files: (none - using workspace defaults)\n")
			}
		} else {
			fmt.Printf("  Bootstrap files: (directory not found - using workspace defaults)\n")
		}
	}
	fmt.Println()
}

// =============================================================================
// Workflow Goal Processor (CLI mode LLM calls)
// =============================================================================

// cliGoalProcessor implements workflow.GoalProcessor using an LLM provider.
type cliGoalProcessor struct {
	provider providers.LLMProvider
	model    string
}

func (p *cliGoalProcessor) ProcessGoal(ctx context.Context, goal string) (string, error) {
	messages := []providers.Message{
		{Role: "user", Content: goal},
	}
	resp, err := p.provider.Chat(ctx, messages, nil, p.model, nil)
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}
	return resp.Content, nil
}

// =============================================================================
// Workflow Commands
// =============================================================================

func workflowCmd() {
	if len(os.Args) < 3 {
		workflowHelp()
		return
	}

	subcommand := os.Args[2]

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	workspace := cfg.WorkspacePath()

	switch subcommand {
	case "list":
		workflowListCmd(workspace, cfg)
	case "show":
		if len(os.Args) < 4 {
			fmt.Println("Usage: pepebot workflow show <name>")
			return
		}
		workflowShowCmd(workspace, cfg, os.Args[3])
	case "run":
		workflowRunCmd(workspace, cfg)
	case "delete", "remove":
		if len(os.Args) < 4 {
			fmt.Println("Usage: pepebot workflow delete <name>")
			return
		}
		workflowDeleteCmd(workspace, cfg, os.Args[3])
	case "validate":
		workflowValidateCmd(workspace, cfg)
	default:
		fmt.Printf("Unknown workflow command: %s\n", subcommand)
		workflowHelp()
	}
}

func workflowHelp() {
	fmt.Println("\nWorkflow commands:")
	fmt.Println("  list                         List all workflows in workspace")
	fmt.Println("  show <name>                  Show workflow details (steps, variables)")
	fmt.Println("  run <name> [options]          Execute a workflow from workspace")
	fmt.Println("    -f, --file <path>           Load workflow from file instead of workspace")
	fmt.Println("    --var key=value             Override a workflow variable (repeatable)")
	fmt.Println("  delete <name>                Delete a workflow from workspace")
	fmt.Println("  validate <name>              Validate workflow structure")
	fmt.Println("    -f, --file <path>           Validate a file instead of workspace workflow")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  pepebot workflow list")
	fmt.Println("  pepebot workflow show my_workflow")
	fmt.Println("  pepebot workflow run my_workflow")
	fmt.Println("  pepebot workflow run my_workflow --var device=emulator-5554 --var query=hello")
	fmt.Println("  pepebot workflow run -f /tmp/test.json")
	fmt.Println("  pepebot workflow run -f /tmp/test.json --var key=value")
	fmt.Println("  pepebot workflow validate my_workflow")
	fmt.Println("  pepebot workflow validate -f /tmp/test.json")
	fmt.Println("  pepebot workflow delete old_workflow")
}

func newWorkflowHelper(workspace string, cfg *config.Config, goalProcessor workflow.GoalProcessor) *workflow.WorkflowHelper {
	registry := tools.NewToolRegistry()
	registry.Register(tools.NewReadFileTool(workspace))
	registry.Register(tools.NewWriteFileTool(workspace))
	registry.Register(tools.NewListDirTool(workspace))
	registry.Register(tools.NewExecTool(workspace))
	registry.Register(tools.NewWebSearchTool(cfg.Tools.Web.Search.APIKey, cfg.Tools.Web.Search.MaxResults))
	registry.Register(tools.NewWebFetchTool(50000))
	registry.Register(tools.NewManageMCPTool(workspace))

	// Platform messaging tools for workflow steps (direct API, no gateway required)
	if cfg.Channels.Telegram.Token != "" {
		registry.Register(tools.NewTelegramSendTool(cfg.Channels.Telegram.Token, workspace))
	}
	if cfg.Channels.Discord.Token != "" {
		registry.Register(tools.NewDiscordSendTool(cfg.Channels.Discord.Token, workspace))
	}
	// WhatsApp: forwards to the running gateway via HTTP (gateway must be running for delivery)
	registry.Register(tools.NewWhatsAppSendViaGateway(cfg.Gateway.Host, cfg.Gateway.Port, workspace))

	if adbHelper, err := tools.NewAdbHelper(workspace); err == nil {
		registry.Register(tools.NewAdbDevicesTool(adbHelper))
		registry.Register(tools.NewAdbShellTool(adbHelper))
		registry.Register(tools.NewAdbTapTool(adbHelper))
		registry.Register(tools.NewAdbInputTextTool(adbHelper))
		registry.Register(tools.NewAdbScreenshotTool(adbHelper))
		registry.Register(tools.NewAdbUIDumpTool(adbHelper))
		registry.Register(tools.NewAdbSwipeTool(adbHelper))
		registry.Register(tools.NewAdbOpenAppTool(adbHelper))
		registry.Register(tools.NewAdbKeyEventTool(adbHelper))
	}

	helper := workflow.NewWorkflowHelper(workspace, registry)
	if goalProcessor != nil {
		helper.SetGoalProcessor(goalProcessor)
	}
	registry.Register(tools.NewWorkflowExecuteTool(helper))
	registry.Register(tools.NewWorkflowSaveTool(helper))
	registry.Register(tools.NewWorkflowListTool(helper))

	return helper
}

func workflowListCmd(workspace string, cfg *config.Config) {
	helper := newWorkflowHelper(workspace, cfg, nil)
	names := helper.ListWorkflows()

	if len(names) == 0 {
		fmt.Printf("No workflows found in %s\n", helper.WorkflowsDir())
		return
	}

	fmt.Println("\nWorkflows:")
	fmt.Println("----------")
	for _, name := range names {
		wf, err := helper.LoadWorkflow(name)
		if err != nil {
			fmt.Printf("  ✗ %-30s (error: %v)\n", name, err)
			continue
		}
		desc := wf.Description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Printf("  %-30s %d steps  %s\n", name, len(wf.Steps), desc)
	}
	fmt.Println()
}

func workflowShowCmd(workspace string, cfg *config.Config, name string) {
	helper := newWorkflowHelper(workspace, cfg, nil)

	wf, err := helper.LoadWorkflow(name)
	if err != nil {
		fmt.Printf("✗ %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n📋 Workflow: %s\n", wf.Name)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if wf.Description != "" {
		fmt.Printf("  Description: %s\n", wf.Description)
	}

	if len(wf.Variables) > 0 {
		fmt.Println("\n  Variables:")
		for k, v := range wf.Variables {
			fmt.Printf("    %-20s = %s\n", k, v)
		}
	}

	fmt.Printf("\n  Steps (%d):\n", len(wf.Steps))
	for i, step := range wf.Steps {
		fmt.Printf("\n  [%d] %s\n", i+1, step.Name)
		switch {
		case step.Tool != "":
			fmt.Printf("      type: tool → %s\n", step.Tool)
			for k, v := range step.Args {
				fmt.Printf("      arg  %-16s = %v\n", k, v)
			}
		case step.Skill != "":
			fmt.Printf("      type: skill → %s\n", step.Skill)
			fmt.Printf("      goal: %s\n", step.Goal)
		case step.Agent != "":
			fmt.Printf("      type: agent → %s\n", step.Agent)
			fmt.Printf("      goal: %s\n", step.Goal)
		default:
			fmt.Printf("      type: goal\n")
			fmt.Printf("      goal: %s\n", step.Goal)
		}
	}
	fmt.Println()
}

func workflowRunCmd(workspace string, cfg *config.Config) {
	args := os.Args[3:]
	workflowName := ""
	filePath := ""
	overrideVars := map[string]string{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		case "--var":
			if i+1 < len(args) {
				parts := strings.SplitN(args[i+1], "=", 2)
				if len(parts) == 2 {
					overrideVars[parts[0]] = parts[1]
				} else {
					fmt.Printf("Warning: --var %q is not in key=value format, skipping\n", args[i+1])
				}
				i++
			}
		default:
			if workflowName == "" && !strings.HasPrefix(args[i], "-") {
				workflowName = args[i]
			}
		}
	}

	if workflowName == "" && filePath == "" {
		fmt.Println("Usage: pepebot workflow run <name> [--var key=value ...]")
		fmt.Println("       pepebot workflow run -f <path> [--var key=value ...]")
		return
	}

	// Create LLM provider for goal step processing
	var goalProc workflow.GoalProcessor
	provider, err := providers.CreateProvider(cfg)
	if err != nil {
		fmt.Printf("Warning: could not create LLM provider for goal steps: %v\n", err)
	} else {
		goalProc = &cliGoalProcessor{
			provider: provider,
			model:    cfg.Agents.Defaults.Model,
		}
	}

	helper := newWorkflowHelper(workspace, cfg, goalProc)

	if len(overrideVars) > 0 {
		fmt.Println("Variables:")
		for k, v := range overrideVars {
			fmt.Printf("  %s = %s\n", k, v)
		}
		fmt.Println()
	}

	ctx := context.Background()
	var result string

	if filePath != "" {
		fmt.Printf("Running workflow from file: %s\n\n", filePath)
		result, err = helper.RunWorkflowFile(ctx, filePath, overrideVars)
	} else {
		fmt.Printf("Running workflow: %s\n\n", workflowName)
		result, err = helper.RunWorkflow(ctx, workflowName, overrideVars)
	}

	if err != nil {
		fmt.Printf("✗ %v\n", err)
		os.Exit(1)
	}
	fmt.Println(result)
}

func workflowDeleteCmd(workspace string, cfg *config.Config, name string) {
	helper := newWorkflowHelper(workspace, cfg, nil)

	if _, err := helper.LoadWorkflow(name); err != nil {
		fmt.Printf("✗ Workflow %q not found: %v\n", name, err)
		os.Exit(1)
	}

	fmt.Printf("Delete workflow %q? (y/n): ", name)
	var response string
	fmt.Scanln(&response)
	if strings.ToLower(strings.TrimSpace(response)) != "y" {
		fmt.Println("Aborted.")
		return
	}

	path := filepath.Join(helper.WorkflowsDir(), name+".json")
	if err := os.Remove(path); err != nil {
		fmt.Printf("✗ Failed to delete: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Deleted workflow %q\n", name)
}

func workflowValidateCmd(workspace string, cfg *config.Config) {
	args := os.Args[3:]
	workflowName := ""
	filePath := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-f", "--file":
			if i+1 < len(args) {
				filePath = args[i+1]
				i++
			}
		default:
			if workflowName == "" && !strings.HasPrefix(args[i], "-") {
				workflowName = args[i]
			}
		}
	}

	if workflowName == "" && filePath == "" {
		fmt.Println("Usage: pepebot workflow validate <name>")
		fmt.Println("       pepebot workflow validate -f <path>")
		return
	}

	helper := newWorkflowHelper(workspace, cfg, nil)

	var wfDef *workflow.WorkflowDefinition
	var loadErr error

	if filePath != "" {
		wfDef, loadErr = helper.LoadWorkflowFile(filePath)
	} else {
		wfDef, loadErr = helper.LoadWorkflow(workflowName)
	}

	if loadErr != nil {
		fmt.Printf("✗ Failed to load: %v\n", loadErr)
		os.Exit(1)
	}

	if err := helper.Validate(wfDef); err != nil {
		fmt.Printf("✗ Validation failed: %v\n", err)
		os.Exit(1)
	}

	source := workflowName
	if filePath != "" {
		source = filePath
	}
	fmt.Printf("✓ Workflow %q is valid (%d steps)\n", source, len(wfDef.Steps))
}

// =============================================================================
// Update Command
// =============================================================================

func updateCmd() {
	// Detect current binary path
	execPath, err := os.Executable()
	if err != nil {
		fmt.Printf("✗ Failed to detect binary path: %v\n", err)
		os.Exit(1)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		fmt.Printf("✗ Failed to resolve symlinks: %v\n", err)
		os.Exit(1)
	}

	// Detect OS/arch for asset naming
	osName := runtime.GOOS
	archName := runtime.GOARCH
	switch archName {
	case "arm":
		archName = "armv7"
	}

	binaryExt := ""
	if osName == "windows" {
		binaryExt = ".exe"
	}

	fmt.Printf("%s pepebot update\n\n", logo)
	fmt.Printf("  Current version: v%s\n", version)
	fmt.Printf("  Binary:          %s\n", execPath)
	fmt.Printf("  Platform:        %s/%s\n\n", osName, archName)

	// Fetch latest release info from GitHub
	fmt.Println("Checking for updates...")
	latestVersion, err := fetchLatestVersion()
	if err != nil {
		fmt.Printf("✗ Failed to check for updates: %v\n", err)
		os.Exit(1)
	}

	// Normalize version (strip leading 'v')
	latestClean := strings.TrimPrefix(latestVersion, "v")
	if latestClean == version {
		fmt.Printf("\n✓ Already up to date (v%s)\n", version)
		return
	}

	fmt.Printf("  Latest version:  %s\n\n", latestVersion)

	// Build download URL
	assetName := fmt.Sprintf("pepebot-%s-%s.tar.gz", osName, archName)
	downloadURL := fmt.Sprintf("https://github.com/pepebot-space/pepebot/releases/download/%s/%s", latestVersion, assetName)
	binaryName := fmt.Sprintf("pepebot-%s-%s%s", osName, archName, binaryExt)

	fmt.Printf("Downloading %s...\n", assetName)

	// Download the tar.gz
	resp, err := http.Get(downloadURL)
	if err != nil {
		fmt.Printf("✗ Failed to download: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("✗ Download failed: HTTP %d\n", resp.StatusCode)
		if resp.StatusCode == 404 {
			fmt.Printf("  Asset not found: %s\n", assetName)
			fmt.Printf("  Check available releases at: https://github.com/pepebot-space/pepebot/releases\n")
		}
		os.Exit(1)
	}

	// Extract binary from tar.gz
	binaryData, err := extractBinaryFromTarGz(resp.Body, binaryName)
	if err != nil {
		fmt.Printf("✗ Failed to extract binary: %v\n", err)
		os.Exit(1)
	}

	// Atomic replace: write to temp file in same directory, then rename
	dir := filepath.Dir(execPath)
	tmpFile, err := os.CreateTemp(dir, "pepebot-update-*")
	if err != nil {
		fmt.Printf("✗ Failed to create temp file: %v\n", err)
		os.Exit(1)
	}
	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(binaryData); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		fmt.Printf("✗ Failed to write update: %v\n", err)
		os.Exit(1)
	}
	tmpFile.Close()

	// Set executable permissions
	if err := os.Chmod(tmpPath, 0755); err != nil {
		os.Remove(tmpPath)
		fmt.Printf("✗ Failed to set permissions: %v\n", err)
		os.Exit(1)
	}

	// Replace the binary
	if err := os.Rename(tmpPath, execPath); err != nil {
		os.Remove(tmpPath)
		fmt.Printf("✗ Failed to replace binary: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n✓ Updated pepebot: v%s → %s\n", version, latestVersion)
}

// fetchLatestVersion queries the GitHub API for the latest release tag.
func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", "https://api.github.com/repos/pepebot-space/pepebot/releases/latest", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in release response")
	}

	return release.TagName, nil
}

// extractBinaryFromTarGz reads a tar.gz stream and returns the contents of the
// file matching binaryName.
func extractBinaryFromTarGz(r io.Reader, binaryName string) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip error: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("tar error: %w", err)
		}

		if header.Typeflag == tar.TypeReg && filepath.Base(header.Name) == binaryName {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, fmt.Errorf("read error: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("binary %q not found in archive", binaryName)
}

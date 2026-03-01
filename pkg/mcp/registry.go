package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type ServerDefinition struct {
	Enabled     bool              `json:"enabled"`
	Transport   string            `json:"transport"`
	Description string            `json:"description,omitempty"`
	URL         string            `json:"url,omitempty"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Source      string            `json:"source,omitempty"`
	Skill       string            `json:"skill,omitempty"`
}

type Registry struct {
	Version string                       `json:"version"`
	Servers map[string]*ServerDefinition `json:"servers"`
}

type RegistryStore struct {
	path string
	mu   sync.RWMutex
}

func NewRegistryStore(workspace string) *RegistryStore {
	return &RegistryStore{path: filepath.Join(workspace, "mcp", "registry.json")}
}

func (s *RegistryStore) defaultRegistry() *Registry {
	return &Registry{
		Version: "1.0",
		Servers: make(map[string]*ServerDefinition),
	}
}

func (s *RegistryStore) Load() (*Registry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reg := s.defaultRegistry()
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return reg, nil
		}
		return nil, fmt.Errorf("failed to read mcp registry: %w", err)
	}

	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("failed to parse mcp registry: %w", err)
	}

	if reg.Servers == nil {
		reg.Servers = make(map[string]*ServerDefinition)
	}

	return reg, nil
}

func (s *RegistryStore) Save(reg *Registry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if reg == nil {
		reg = s.defaultRegistry()
	}
	if reg.Version == "" {
		reg.Version = "1.0"
	}
	if reg.Servers == nil {
		reg.Servers = make(map[string]*ServerDefinition)
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return fmt.Errorf("failed to create mcp directory: %w", err)
	}

	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mcp registry: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0644); err != nil {
		return fmt.Errorf("failed to write mcp registry: %w", err)
	}

	return nil
}

func (s *RegistryStore) List() (map[string]*ServerDefinition, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}

	result := make(map[string]*ServerDefinition, len(reg.Servers))
	for name, def := range reg.Servers {
		copyDef := *def
		result[name] = &copyDef
	}

	return result, nil
}

func (s *RegistryStore) AddOrUpdate(name string, def *ServerDefinition) error {
	if err := ValidateServerDefinition(name, def); err != nil {
		return err
	}

	reg, err := s.Load()
	if err != nil {
		return err
	}

	reg.Servers[name] = normalizeDefinition(def)
	return s.Save(reg)
}

func (s *RegistryStore) Remove(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("mcp name cannot be empty")
	}

	reg, err := s.Load()
	if err != nil {
		return err
	}

	if _, ok := reg.Servers[name]; !ok {
		return fmt.Errorf("mcp server '%s' not found", name)
	}

	delete(reg.Servers, name)
	return s.Save(reg)
}

func (s *RegistryStore) UpsertFromSkill(skillName, serverName string, def *ServerDefinition) error {
	if def == nil {
		return fmt.Errorf("mcp server definition cannot be nil")
	}

	copyDef := *def
	copyDef.Source = "skill"
	copyDef.Skill = skillName
	if !copyDef.Enabled {
		copyDef.Enabled = true
	}

	return s.AddOrUpdate(serverName, &copyDef)
}

func ValidateServerDefinition(name string, def *ServerDefinition) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("mcp name cannot be empty")
	}
	if def == nil {
		return fmt.Errorf("mcp definition is required")
	}

	transport := strings.ToLower(strings.TrimSpace(def.Transport))
	switch transport {
	case "stdio":
		if strings.TrimSpace(def.Command) == "" {
			return fmt.Errorf("stdio transport requires command")
		}
	case "http", "sse":
		if strings.TrimSpace(def.URL) == "" {
			return fmt.Errorf("%s transport requires url", transport)
		}
	default:
		return fmt.Errorf("unsupported mcp transport '%s' (supported: stdio, sse, http)", def.Transport)
	}

	return nil
}

func normalizeDefinition(def *ServerDefinition) *ServerDefinition {
	copyDef := *def
	copyDef.Transport = strings.ToLower(strings.TrimSpace(copyDef.Transport))
	copyDef.URL = strings.TrimSpace(copyDef.URL)
	copyDef.Command = strings.TrimSpace(copyDef.Command)
	if copyDef.Env == nil {
		copyDef.Env = map[string]string{}
	}
	if copyDef.Headers == nil {
		copyDef.Headers = map[string]string{}
	}
	if copyDef.Args == nil {
		copyDef.Args = []string{}
	}
	return &copyDef
}

func SortedServerNames(servers map[string]*ServerDefinition) []string {
	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

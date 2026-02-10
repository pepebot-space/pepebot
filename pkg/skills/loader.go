package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type SkillMetadata struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Always      bool               `json:"always"`
	Requires    *SkillRequirements `json:"requires,omitempty"`
}

type SkillRequirements struct {
	Bins []string `json:"bins"`
	Env  []string `json:"env"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Missing     string `json:"missing,omitempty"`
}

type SkillsLoader struct {
	workspace       string
	workspaceSkills string
	builtinSkills   string
}

func NewSkillsLoader(workspace string, builtinSkills string) *SkillsLoader {
	return &SkillsLoader{
		workspace:       workspace,
		workspaceSkills: filepath.Join(workspace, "skills"),
		builtinSkills:   builtinSkills,
	}
}

func (sl *SkillsLoader) ListSkills(filterUnavailable bool) []SkillInfo {
	skills := make([]SkillInfo, 0)

	if sl.workspaceSkills != "" {
		if dirs, err := os.ReadDir(sl.workspaceSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					skillFile := filepath.Join(sl.workspaceSkills, dir.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						info := SkillInfo{
							Name:   dir.Name(),
							Path:   skillFile,
							Source: "workspace",
						}
						metadata := sl.getSkillMetadata(skillFile)
						if metadata != nil {
							info.Description = metadata.Description
							info.Available = sl.checkRequirements(metadata.Requires)
							if !info.Available {
								info.Missing = sl.getMissingRequirements(metadata.Requires)
							}
						} else {
							info.Available = true
						}
						skills = append(skills, info)
					}
				}
			}
		}
	}

	if sl.builtinSkills != "" {
		if dirs, err := os.ReadDir(sl.builtinSkills); err == nil {
			for _, dir := range dirs {
				if dir.IsDir() {
					skillFile := filepath.Join(sl.builtinSkills, dir.Name(), "SKILL.md")
					if _, err := os.Stat(skillFile); err == nil {
						exists := false
						for _, s := range skills {
							if s.Name == dir.Name() && s.Source == "workspace" {
								exists = true
								break
							}
						}
						if exists {
							continue
						}

						info := SkillInfo{
							Name:   dir.Name(),
							Path:   skillFile,
							Source: "builtin",
						}
						metadata := sl.getSkillMetadata(skillFile)
						if metadata != nil {
							info.Description = metadata.Description
							info.Available = sl.checkRequirements(metadata.Requires)
							if !info.Available {
								info.Missing = sl.getMissingRequirements(metadata.Requires)
							}
						} else {
							info.Available = true
						}
						skills = append(skills, info)
					}
				}
			}
		}
	}

	if filterUnavailable {
		filtered := make([]SkillInfo, 0)
		for _, s := range skills {
			if s.Available {
				filtered = append(filtered, s)
			}
		}
		return filtered
	}

	return skills
}

func (sl *SkillsLoader) LoadSkill(name string) (string, bool) {
	if sl.workspaceSkills != "" {
		skillFile := filepath.Join(sl.workspaceSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	if sl.builtinSkills != "" {
		skillFile := filepath.Join(sl.builtinSkills, name, "SKILL.md")
		if content, err := os.ReadFile(skillFile); err == nil {
			return sl.stripFrontmatter(string(content)), true
		}
	}

	return "", false
}

func (sl *SkillsLoader) LoadSkillsForContext(skillNames []string) string {
	if len(skillNames) == 0 {
		return ""
	}

	var parts []string
	for _, name := range skillNames {
		content, ok := sl.LoadSkill(name)
		if ok {
			parts = append(parts, fmt.Sprintf("### Skill: %s\n\n%s", name, content))
		}
	}

	return strings.Join(parts, "\n\n---\n\n")
}

func (sl *SkillsLoader) BuildSkillsSummary() string {
	allSkills := sl.ListSkills(false)
	if len(allSkills) == 0 {
		return ""
	}

	var lines []string
	lines = append(lines, "<skills>")
	for _, s := range allSkills {
		escapedName := escapeXML(s.Name)
		escapedDesc := escapeXML(s.Description)
		escapedPath := escapeXML(s.Path)

		available := "true"
		if !s.Available {
			available = "false"
		}

		lines = append(lines, fmt.Sprintf("  <skill available=\"%s\">", available))
		lines = append(lines, fmt.Sprintf("    <name>%s</name>", escapedName))
		lines = append(lines, fmt.Sprintf("    <description>%s</description>", escapedDesc))
		lines = append(lines, fmt.Sprintf("    <location>%s</location>", escapedPath))

		if !s.Available && s.Missing != "" {
			escapedMissing := escapeXML(s.Missing)
			lines = append(lines, fmt.Sprintf("    <requires>%s</requires>", escapedMissing))
		}

		lines = append(lines, "  </skill>")
	}
	lines = append(lines, "</skills>")

	return strings.Join(lines, "\n")
}

func (sl *SkillsLoader) GetAlwaysSkills() []string {
	skills := sl.ListSkills(true)
	var always []string
	for _, s := range skills {
		metadata := sl.getSkillMetadata(s.Path)
		if metadata != nil && metadata.Always {
			always = append(always, s.Name)
		}
	}
	return always
}

func (sl *SkillsLoader) getSkillMetadata(skillPath string) *SkillMetadata {
	content, err := os.ReadFile(skillPath)
	if err != nil {
		return nil
	}

	frontmatter := sl.extractFrontmatter(string(content))
	if frontmatter == "" {
		return &SkillMetadata{
			Name: filepath.Base(filepath.Dir(skillPath)),
		}
	}

	var metadata struct {
		Name        string             `json:"name"`
		Description string             `json:"description"`
		Always      bool               `json:"always"`
		Requires    *SkillRequirements `json:"requires"`
	}

	if err := json.Unmarshal([]byte(frontmatter), &metadata); err != nil {
		return nil
	}

	return &SkillMetadata{
		Name:        metadata.Name,
		Description: metadata.Description,
		Always:      metadata.Always,
		Requires:    metadata.Requires,
	}
}

func (sl *SkillsLoader) extractFrontmatter(content string) string {
	re := regexp.MustCompile(`^---\n(.*?)\n---`)
	match := re.FindStringSubmatch(content)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func (sl *SkillsLoader) stripFrontmatter(content string) string {
	re := regexp.MustCompile(`^---\n.*?\n---\n`)
	return re.ReplaceAllString(content, "")
}

func (sl *SkillsLoader) checkRequirements(requires *SkillRequirements) bool {
	if requires == nil {
		return true
	}

	for _, bin := range requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		} else {
			return true
		}
	}

	for _, env := range requires.Env {
		if os.Getenv(env) == "" {
			return false
		}
	}

	return true
}

func (sl *SkillsLoader) getMissingRequirements(requires *SkillRequirements) string {
	if requires == nil {
		return ""
	}

	var missing []string
	for _, bin := range requires.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, fmt.Sprintf("CLI: %s", bin))
		}
	}

	for _, env := range requires.Env {
		if os.Getenv(env) == "" {
			missing = append(missing, fmt.Sprintf("ENV: %s", env))
		}
	}

	return strings.Join(missing, ", ")
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

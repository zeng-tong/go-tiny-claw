package context

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Skill 定义了从 SKILL.md 中解析出的标准化技能结构
// 引擎启动时可以将 Name 的 Description 告诉 LLM。只有当 LLM 明确判定任务需要技能时，再加载完整的 Markdown 正文。（渐进式披露）
type Skill struct {
	Name        string
	Description string
	Body        string // 正文指令
}

// SkillLoader 负责从本地文件系统中加载并解析符合规范的技能模板
type SkillLoader struct {
	workDir string
}

func NewSkillLoader(workDir string) *SkillLoader {
	return &SkillLoader{
		workDir: workDir,
	}
}

// LoadAll 扫描 .claw/skills 目录，解析所有 SKILL.md ，并格式化为字符串准备注入 Context
func (s *SkillLoader) LoadAll() string {
	skillBaseDir := filepath.Join(s.workDir, ".claw", "skills")

	// 如果目录不存在，说明当前工作区没有配置技能，静默返回
	if _, err := os.Stat(skillBaseDir); os.IsNotExist(err) {
		return ""
	}

	var skillBuilder strings.Builder
	skillBuilder.WriteString("\n### 可用专业技能 （Agent Skills）\n")
	skillBuilder.WriteString("以下是你拥有的标准化外挂技能，请在符合 description 描述的场景下严格遵循其正文指令: \n\n")
	err := filepath.WalkDir(skillBaseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// 仅处理名为 SKILL.md 的文件
		if !d.IsDir() && d.Name() == "SKILL.md" {
			content, err := ioutil.ReadFile(path)
			if err == nil {
				skill := parseSkillMD(string(content))
				skillBuilder.WriteString(fmt.Sprintf("### 技能名称：%s\n", skill.Name))
				skillBuilder.WriteString(fmt.Sprintf("**触发条件**：%s\n\n", skill.Description))
				skillBuilder.WriteString("**执行指南**:\n")
				skillBuilder.WriteString(skill.Body)
				skillBuilder.WriteString("\n\n---\n")
			}
		}
		return nil
	})
	if err != nil || skillBuilder.Len() < 100 {
		return ""
	}
	return skillBuilder.String()
}

func parseSkillMD(content string) Skill {
	skill := Skill{
		Name:        "Unknown Skill",
		Description: "No Description provided",
		Body:        content, // 默认将全量内容作为 Body
	}
	// 简单解析 YAML Frontmatter 以 --- 包裹
	if strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) == 3 {
			frontmatter := parts[1]
			skill.Body = strings.TrimSpace(parts[2])

			lines := strings.Split(frontmatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "name:") {
					skill.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				} else if strings.HasPrefix(line, "description:") {
					skill.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				}
			}
		}
	}
	return skill
}

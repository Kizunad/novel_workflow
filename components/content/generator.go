package content

import "errors"

type Generator struct {
	novelDir string
}

func NewGenerator(novelDir string) *Generator {
	return &Generator{
		novelDir: novelDir,
	}
}

// loadChapter 加载最新章节内容
func (g *Generator) loadChapter() (string, error) {
	manager := NewChapterManager(g.novelDir)
	return manager.GetLatestChapterContent()
}

// loadAllChapters 读取所有章节并拼接成一个字符串
func (g *Generator) loadAllChapters() (string, error) {
	manager := NewChapterManager(g.novelDir)
	return manager.GetAllChaptersContent()
}

func (g *Generator) Generate() (string, error) {
	// 尝试加载所有章节
	content, err := g.loadAllChapters()
	if err != nil {
		// 如果是没有章节的错误，返回空字符串（正常情况）
		if errors.Is(err, ErrNoChapterFiles) || errors.Is(err, ErrNoChapters) {
			content = ""
		} else {
			// 其他错误返回错误信息
			return "", err
		}
	}

	return content, nil
}

// GetNovelDir 获取小说目录路径
func (g *Generator) GetNovelDir() string {
	return g.novelDir
}

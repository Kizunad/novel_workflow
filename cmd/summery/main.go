package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
	"github.com/Kizunad/modular-workflow-v2/queue"
)

// ChapterData JSON章节数据结构
type ChapterData struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	Content   []struct {
		ParagraphID int    `json:"paragraph_id"`
		Text        string `json:"text"`
	} `json:"content"`
}

func main() {
	fmt.Println("=== 测试 Summarizer 功能 - 使用真实章节数据 ===")

	// 初始化日志
	log := logger.New()

	// 加载配置
	cfg, err := config.NewLoader().Load("../../config.yaml")
	if err != nil {
		log.Error(fmt.Sprintf("加载配置失败: %v", err))
		os.Exit(1)
	}

	// 初始化LLM管理器
	llmManager := providers.NewManager(cfg, *log)

	// 初始化消息队列
	mq, err := queue.InitQueue(&cfg.MessageQueue, llmManager, log)
	if err != nil {
		log.Error(fmt.Sprintf("初始化消息队列失败: %v", err))
		os.Exit(1)
	}

	if mq == nil {
		log.Error("消息队列被禁用，请在config.yaml中启用")
		os.Exit(1)
	}

	// 启动消息队列
	ctx := context.Background()
	if err := mq.Start(ctx); err != nil {
		log.Error(fmt.Sprintf("启动消息队列失败: %v", err))
		os.Exit(1)
	}

	// 使用真实数据路径
	novelPath := "../../../novels/creation"
	fmt.Printf("小说路径: %s\n", novelPath)

	// 查找所有章节文件
	files, err := filepath.Glob(filepath.Join(novelPath, "example_chapter_*.json"))
	if err != nil {
		log.Error(fmt.Sprintf("查找章节文件失败: %v", err))
		os.Exit(1)
	}

	if len(files) == 0 {
		log.Error("未找到章节文件")
		os.Exit(1)
	}

	fmt.Printf("找到 %d 个章节文件\n", len(files))

	// 逐个处理章节
	for i, file := range files {
		fmt.Printf("\n--- 处理第 %d/%d 个章节: %s ---\n", i+1, len(files), filepath.Base(file))

		// 读取章节文件
		data, err := os.ReadFile(file)
		if err != nil {
			log.Warn(fmt.Sprintf("读取文件 %s 失败: %v", file, err))
			continue
		}

		// 解析JSON
		var chapterData ChapterData
		if err := json.Unmarshal(data, &chapterData); err != nil {
			log.Warn(fmt.Sprintf("解析JSON文件 %s 失败: %v", file, err))
			continue
		}

		// 组装章节内容
		contentBuilder := strings.Builder{}
		contentBuilder.WriteString(fmt.Sprintf("=== %s ===\n", chapterData.Title))

		for _, paragraph := range chapterData.Content {
			contentBuilder.WriteString(paragraph.Text)
			contentBuilder.WriteString("\n\n")
		}

		content := contentBuilder.String()
		fmt.Printf("章节标题: %s\n", chapterData.Title)
		fmt.Printf("内容长度: %d字符\n", len(content))

		// 创建摘要任务
		task := queue.CreateSummarizeTask(novelPath, content)
		fmt.Printf("创建任务: %s\n", task.GetID())

		// 提交任务到队列
		if err := mq.Enqueue(task); err != nil {
			log.Warn(fmt.Sprintf("提交任务失败: %v", err))
			continue
		}

		fmt.Println("任务已提交到队列")

		// 等待一会儿，让队列有时间处理
		time.Sleep(2 * time.Second)
	}

	fmt.Printf("\n=== 所有章节处理完成 ===\n")
	fmt.Printf("请检查 %s/index.json 文件\n", novelPath)

	// 等待队列处理完成
	mq.WaitUntilComplete()
	
	// 优雅关闭队列
	if err := mq.Shutdown(5 * time.Second); err != nil {
		log.Warn(fmt.Sprintf("关闭队列失败: %v", err))
	}

}

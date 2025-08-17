package template

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/HildaM/logs/slog"
)

// GetPromptTemplate 加载并返回一个提示模板
func GetPromptTemplate(ctx context.Context, promptName string) (string, error) {
	// 获取当前路径
	dir, err := os.Getwd()
	if err != nil {
		msg := fmt.Errorf("GetPromptTemplate failed, get current working directory, err: %w", err)
		slog.Error(msg.Error())
		return "", msg
	}

	// 构造文件路径
	templatePath := filepath.Join(dir, "prompts", fmt.Sprintf("%s.md", promptName))

	// 读取文件内容
	content, err := ioutil.ReadFile(templatePath)
	if err != nil {
		msg := fmt.Errorf("GetPromptTemplate failed, read template file, err: %w", err)
		slog.Error(msg.Error())
		return "", msg
	}
	return string(content), nil
}

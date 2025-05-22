package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// ProviderConfig 定义单个提供者的配置
type ProviderConfig struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret,omitempty"`
	APIURL    string `json:"api_url,omitempty"`
	ModelName string `json:"model_name,omitempty"`
}

// Config 定义配置结构
type Config struct {
	DefaultProvider string                    `json:"default_provider"`
	Providers       map[string]ProviderConfig `json:"providers"`
}

// LoadConfig 从文件加载配置
func LoadConfig() (*Config, error) {
	config := &Config{
		DefaultProvider: "deepseek",
		Providers: map[string]ProviderConfig{
			"deepseek": {
				APIKey: "your_deepseek_api_key_here",
			},
			"siliconflow": {
				APIKey: "your_siliconflow_api_key_here",
			},
			"aliyun": {
				APIKey:    "your_aliyun_api_key_here",
				APISecret: "your_aliyun_api_secret_here",
			},
			"github": {
				APIKey:    "your_github_api_key_here",
				APIURL:    "https://models.inference.ai.azure.com/chat/completions",
				ModelName: "gpt-4o",
			},
		},
	}

	// 尝试读取配置文件
	data, err := os.ReadFile("config.json")
	if err != nil {
		if os.IsNotExist(err) {
			// 如果文件不存在，创建默认配置文件
			if err := saveConfig(config); err != nil {
				return nil, fmt.Errorf("创建默认配置文件失败: %v", err)
			}
			return config, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 解析配置文件
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	return config, nil
}

// saveConfig 保存配置到文件
func saveConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile("config.json", data, 0644)
}

// GetProviderConfig 获取指定提供者的配置
func (c *Config) GetProviderConfig(providerType string) (ProviderConfig, error) {
	if providerType == "" {
		providerType = c.DefaultProvider
	}

	config, exists := c.Providers[providerType]
	if !exists {
		return ProviderConfig{}, fmt.Errorf("不支持的模型类型: %s", providerType)
	}
	return config, nil
}

# 智能文件整理程序

这是一个使用 Go 语言开发的智能文件整理程序，它可以根据文件名称的相似性，使用大模型对文件进行智能分类。

## 功能特点

- 支持扫描指定文件夹下的所有文件和子文件夹
- 使用大模型对文件进行智能分类
- 支持多种大模型接口（Deepseek、SiliconFlow、阿里云）
- 自动创建分类文件夹并移动文件
- 配置文件支持，方便管理 API 密钥

## 使用方法

1. 确保已安装 Go 1.21 或更高版本
2. 克隆本仓库
3. 运行 `go mod tidy` 安装依赖
4. 配置 `config.json` 文件，设置您的大模型 API 密钥
5. 运行程序：`go run .`

## 配置说明

在 `config.json` 文件中配置您的大模型 API 信息：

```json
{
    "provider_type": "deepseek",
    "api_keys": {
        "api_key": "您的API密钥",
        "api_secret": "您的API密钥（如果需要）"
    }
}
```

支持的 provider_type 值：
- deepseek
- siliconflow
- aliyun

## 注意事项

- 请确保您有足够的 API 调用额度
- 建议在移动文件前先备份重要数据
- 程序会自动跳过根目录，只处理子文件夹和文件 
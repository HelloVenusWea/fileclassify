package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LLMProvider 定义大模型接口
type LLMProvider interface {
	ClassifyFiles(files []FileInfo) (map[string][]FileInfo, error)
	GetConfig() (string, string, string) // 返回 modelName, apiURL, apiKey
}

// DeepseekProvider Deepseek模型实现
type DeepseekProvider struct {
	APIKey    string
	APIURL    string
	ModelName string
}

// SiliconFlowProvider SiliconFlow模型实现
type SiliconFlowProvider struct {
	APIKey    string
	APIURL    string
	ModelName string
}

// AliyunProvider 阿里云模型实现
type AliyunProvider struct {
	APIKey    string
	APISecret string
	APIURL    string
	ModelName string
}

// GitHubProvider GitHub模型实现
type GitHubProvider struct {
	APIKey    string
	APIURL    string
	ModelName string
}

// 添加通用的API请求结构
type APIRequest struct {
	Model     string              `json:"model"`
	Messages  []map[string]string `json:"messages"`
	MaxTokens int                 `json:"max_tokens"`
}

// 添加通用的API响应结构
type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error map[string]interface{} `json:"error,omitempty"`
}

// NewLLMProvider 创建大模型提供者
func NewLLMProvider(providerType string, config map[string]string) (LLMProvider, error) {
	switch providerType {
	case "deepseek":
		return &DeepseekProvider{
			APIKey:    config["api_key"],
			APIURL:    config["api_url"],
			ModelName: config["model_name"],
		}, nil
	case "siliconflow":
		return &SiliconFlowProvider{
			APIKey:    config["api_key"],
			APIURL:    config["api_url"],
			ModelName: config["model_name"],
		}, nil
	case "aliyun":
		return &AliyunProvider{
			APIKey:    config["api_key"],
			APISecret: config["api_secret"],
			APIURL:    config["api_url"],
			ModelName: config["model_name"],
		}, nil
	case "github":
		return &GitHubProvider{
			APIKey:    config["api_key"],
			APIURL:    config["api_url"],
			ModelName: config["model_name"],
		}, nil
	default:
		return nil, fmt.Errorf("不支持的模型类型: %s", providerType)
	}
}

// 提取JSON内容的辅助函数
func extractJSONFromContent(content string) string {
	content = strings.TrimSpace(content)

	// 去除markdown代码块包裹
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSpace(content)
	}
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSpace(content)
	}
	if strings.HasSuffix(content, "```") {
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// 查找第一个 { 和最后一个 } 之间的内容
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start == -1 || end == -1 || end <= start {
		return content
	}

	// 提取JSON内容
	jsonContent := content[start : end+1]

	// 验证提取的JSON是否完整
	if !isValidJSON(jsonContent) {
		fmt.Printf("警告：提取的JSON内容可能不完整: %s\n", jsonContent)
		// 尝试修复不完整的JSON
		jsonContent = fixIncompleteJSON(jsonContent)
	}

	return jsonContent
}

// 添加修复不完整JSON的函数
func fixIncompleteJSON(content string) string {
	content = strings.TrimSpace(content)

	// 如果内容为空，直接返回
	if content == "" {
		return content
	}

	// 检查是否以 { 开头
	if !strings.HasPrefix(content, "{") {
		content = "{" + content
	}

	// 检查是否以 } 结尾
	if !strings.HasSuffix(content, "}") {
		content = content + "}"
	}

	// 检查并修复未闭合的数组
	openBrackets := 0
	closeBrackets := 0
	for _, char := range content {
		if char == '[' {
			openBrackets++
		} else if char == ']' {
			closeBrackets++
		}
	}

	// 如果数组未闭合，添加缺失的闭合括号
	if openBrackets > closeBrackets {
		content = content + strings.Repeat("]", openBrackets-closeBrackets)
	}

	// 检查并修复未闭合的对象
	openBraces := 0
	closeBraces := 0
	for _, char := range content {
		if char == '{' {
			openBraces++
		} else if char == '}' {
			closeBraces++
		}
	}

	// 如果对象未闭合，添加缺失的闭合括号
	if openBraces > closeBraces {
		content = content + strings.Repeat("}", openBraces-closeBraces)
	}

	// 检查并修复未闭合的字符串
	quotes := 0
	for _, char := range content {
		if char == '"' {
			quotes++
		}
	}

	// 如果字符串未闭合，添加缺失的引号
	if quotes%2 != 0 {
		content = content + "\""
	}

	return content
}

// 添加JSON验证函数
func isValidJSON(content string) bool {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") || !strings.HasSuffix(content, "}") {
		return false
	}

	// 检查括号是否匹配
	var stack []rune
	inString := false
	escaped := false

	for _, char := range content {
		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' && !escaped {
			inString = !inString
			continue
		}

		if inString {
			continue
		}

		switch char {
		case '{', '[':
			stack = append(stack, char)
		case '}':
			if len(stack) == 0 || stack[len(stack)-1] != '{' {
				return false
			}
			stack = stack[:len(stack)-1]
		case ']':
			if len(stack) == 0 || stack[len(stack)-1] != '[' {
				return false
			}
			stack = stack[:len(stack)-1]
		}
	}

	return len(stack) == 0 && !inString
}

// 将文件列表分块，每块最多包含100个文件
func splitFileList(files []FileInfo) [][]FileInfo {
	const maxFilesPerChunk = 150 // 减小每批处理的文件数量
	var chunks [][]FileInfo
	for i := 0; i < len(files); i += maxFilesPerChunk {
		end := i + maxFilesPerChunk
		if end > len(files) {
			end = len(files)
		}
		chunks = append(chunks, files[i:end])
	}
	return chunks
}

// 合并分类结果
func mergeClassificationResults(results []map[string][]FileInfo) map[string][]FileInfo {
	merged := make(map[string][]FileInfo)
	for _, result := range results {
		for category, files := range result {
			merged[category] = append(merged[category], files...)
		}
	}
	return merged
}

// 添加重试机制的辅助函数
func retryWithBackoff(operation func() error, maxRetries int) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = operation()
		if err == nil {
			return nil
		}

		// 计算退避时间（指数退避）
		backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
		fmt.Printf("操作失败，%d秒后重试 (第%d次重试): %v\n", int(backoff.Seconds()), i+1, err)
		time.Sleep(backoff)
	}
	return fmt.Errorf("在%d次重试后仍然失败: %v", maxRetries, err)
}

// 添加通用的分类处理函数
func processClassificationChunk(chunk []FileInfo, provider LLMProvider, processedFiles map[string]bool) (map[string][]FileInfo, error) {
	// 构建文件列表字符串
	var fileList strings.Builder
	for _, file := range chunk {
		fileList.WriteString(fmt.Sprintf("- %s\n", file.Path))
	}

	// 构建提示词
	prompt := fmt.Sprintf(`请根据以下文件列表，将文件按照相似性进行分类。请使用中文命名分类，并返回JSON格式的分类结果。
文件列表：
%s

请按照以下JSON格式返回分类结果：
{
    "分类名称1": ["文件路径1", "文件路径2", ...],
    "分类名称2": ["文件路径1", "文件路径2", ...],
    ...
}

注意：
1. 请确保返回的是有效的JSON格式，不要包含任何其他文本
2. 请确保所有文件都被分类，不要遗漏任何文件
3. 如果文件内容不明确，可以将其归类到"其他"类别`, fileList.String())

	// 获取提供者配置
	modelName, apiURL, apiKey := provider.GetConfig()

	// 构建API请求
	request := APIRequest{
		Model: modelName,
		Messages: []map[string]string{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		MaxTokens: 8192,
	}

	// 调用API
	response, err := callAPI(apiURL, apiKey, request)
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %v", err)
	}

	// 解析分类结果
	content := extractJSONFromContent(response.Choices[0].Message.Content)
	fmt.Printf("提取的JSON内容: %s\n", content)

	// 尝试修复不完整的JSON
	content = fixIncompleteJSON(content)
	fmt.Printf("修复后的JSON内容: %s\n", content)

	// 检查JSON内容是否完整
	if !isValidJSON(content) {
		return nil, fmt.Errorf("API返回的JSON内容不完整，请检查API响应")
	}

	var categories map[string][]string
	if err := json.Unmarshal([]byte(content), &categories); err != nil {
		return nil, fmt.Errorf("解析分类结果失败: %v\nJSON内容: %s", err, content)
	}

	// 验证分类结果
	if len(categories) == 0 {
		return nil, fmt.Errorf("API返回的分类结果为空")
	}

	// 检查是否所有文件都被分类
	classifiedPaths := make(map[string]bool)
	for _, paths := range categories {
		for _, path := range paths {
			classifiedPaths[path] = true
		}
	}

	// 将分类结果转换为FileInfo格式
	classifiedFiles := make(map[string][]FileInfo)
	for category, filePaths := range categories {
		classifiedFiles[category] = make([]FileInfo, 0)
		for _, path := range filePaths {
			for _, file := range chunk {
				if file.Path == path {
					file.Category = category
					classifiedFiles[category] = append(classifiedFiles[category], file)
					processedFiles[file.Path] = true
					break
				}
			}
		}
	}

	return classifiedFiles, nil
}

// 添加并发处理函数
func processChunksConcurrently(chunks [][]FileInfo, provider LLMProvider, processedFiles map[string]bool) ([]map[string][]FileInfo, error) {
	var (
		allResults []map[string][]FileInfo
		mu         sync.Mutex
		wg         sync.WaitGroup
		errChan    = make(chan error, len(chunks))
	)

	for i, chunk := range chunks {
		wg.Add(1)
		go func(i int, chunk []FileInfo) {
			defer wg.Done()
			fmt.Printf("正在处理第 %d/%d 批文件...\n", i+1, len(chunks))
			fmt.Printf("本批次包含 %d 个文件\n", len(chunk))

			result, err := processClassificationChunk(chunk, provider, processedFiles)
			if err != nil {
				errChan <- fmt.Errorf("处理第%d批文件失败: %v", i+1, err)
				return
			}

			mu.Lock()
			allResults = append(allResults, result)
			mu.Unlock()
		}(i, chunk)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(errChan)

	// 检查是否有错误发生
	for err := range errChan {
		return nil, err
	}

	return allResults, nil
}

// 修改各个提供者的ClassifyFiles方法
func (p *DeepseekProvider) ClassifyFiles(files []FileInfo) (map[string][]FileInfo, error) {
	// 将文件列表分成较小的批次
	chunks := splitFileList(files)

	// 创建一个map来跟踪所有文件
	processedFiles := make(map[string]bool)
	for _, file := range files {
		processedFiles[file.Path] = false
	}

	// 并发处理所有批次
	allResults, err := processChunksConcurrently(chunks, p, processedFiles)
	if err != nil {
		return nil, err
	}

	// 处理未分类的文件
	unclassifiedFiles := handleUnclassifiedFiles(files, processedFiles)
	if len(unclassifiedFiles) > 0 {
		allResults = append(allResults, unclassifiedFiles)
	}

	return mergeClassificationResults(allResults), nil
}

// 处理未分类文件的函数
func handleUnclassifiedFiles(files []FileInfo, processedFiles map[string]bool) map[string][]FileInfo {
	var unprocessedFiles []string
	for path, processed := range processedFiles {
		if !processed {
			unprocessedFiles = append(unprocessedFiles, path)
		}
	}

	if len(unprocessedFiles) > 0 {
		fmt.Printf("\n警告：发现 %d 个文件未被分类：\n", len(unprocessedFiles))
		for _, path := range unprocessedFiles {
			fmt.Printf("- %s\n", path)
		}

		unclassifiedFiles := make(map[string][]FileInfo)
		unclassifiedFiles["未分类"] = make([]FileInfo, 0)
		for _, path := range unprocessedFiles {
			for _, file := range files {
				if file.Path == path {
					file.Category = "未分类"
					unclassifiedFiles["未分类"] = append(unclassifiedFiles["未分类"], file)
					break
				}
			}
		}
		return unclassifiedFiles
	}

	return nil
}

// 修改callAPI函数，增加重试机制
func callAPI(url string, apiKey string, payload interface{}) (*APIResponse, error) {
	var response *APIResponse
	var err error

	// 使用指数退避重试
	for i := 0; i < 3; i++ {
		response, err = doAPICall(url, apiKey, payload)
		if err == nil {
			return response, nil
		}

		// 如果是JSON解析错误，直接返回
		if strings.Contains(err.Error(), "JSON") {
			return nil, err
		}

		// 计算退避时间
		backoff := time.Duration(math.Pow(2, float64(i))) * time.Second
		fmt.Printf("API调用失败，%d秒后重试 (第%d次重试): %v\n", int(backoff.Seconds()), i+1, err)
		time.Sleep(backoff)
	}

	return nil, fmt.Errorf("在3次重试后仍然失败: %v", err)
}

// 添加实际的API调用函数
func doAPICall(url string, apiKey string, payload interface{}) (*APIResponse, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %v", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{
		Timeout: 180 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API请求失败，状态码: %d，响应: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	// 打印原始响应以便调试
	fmt.Printf("API响应状态码: %d\n", resp.StatusCode)
	fmt.Printf("API响应内容: %s\n", string(body))

	var apiResponse APIResponse
	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if apiResponse.Error != nil {
		return nil, fmt.Errorf("API返回错误: %v", apiResponse.Error)
	}

	return &apiResponse, nil
}

func (p *SiliconFlowProvider) ClassifyFiles(files []FileInfo) (map[string][]FileInfo, error) {
	// 将文件列表分成较小的批次
	chunks := splitFileList(files)

	// 创建一个map来跟踪所有文件
	processedFiles := make(map[string]bool)
	for _, file := range files {
		processedFiles[file.Path] = false
	}

	// 并发处理所有批次
	allResults, err := processChunksConcurrently(chunks, p, processedFiles)
	if err != nil {
		return nil, err
	}

	// 处理未分类的文件
	unclassifiedFiles := handleUnclassifiedFiles(files, processedFiles)
	if len(unclassifiedFiles) > 0 {
		allResults = append(allResults, unclassifiedFiles)
	}

	return mergeClassificationResults(allResults), nil
}

func (p *AliyunProvider) ClassifyFiles(files []FileInfo) (map[string][]FileInfo, error) {
	// 将文件列表分成较小的批次
	chunks := splitFileList(files)

	// 创建一个map来跟踪所有文件
	processedFiles := make(map[string]bool)
	for _, file := range files {
		processedFiles[file.Path] = false
	}

	// 并发处理所有批次
	allResults, err := processChunksConcurrently(chunks, p, processedFiles)
	if err != nil {
		return nil, err
	}

	// 处理未分类的文件
	unclassifiedFiles := handleUnclassifiedFiles(files, processedFiles)
	if len(unclassifiedFiles) > 0 {
		allResults = append(allResults, unclassifiedFiles)
	}

	return mergeClassificationResults(allResults), nil
}

func (p *GitHubProvider) ClassifyFiles(files []FileInfo) (map[string][]FileInfo, error) {
	// 将文件列表分成较小的批次
	chunks := splitFileList(files)

	// 创建一个map来跟踪所有文件
	processedFiles := make(map[string]bool)
	for _, file := range files {
		processedFiles[file.Path] = false
	}

	// 并发处理所有批次
	allResults, err := processChunksConcurrently(chunks, p, processedFiles)
	if err != nil {
		return nil, err
	}

	// 处理未分类的文件
	unclassifiedFiles := handleUnclassifiedFiles(files, processedFiles)
	if len(unclassifiedFiles) > 0 {
		allResults = append(allResults, unclassifiedFiles)
	}

	return mergeClassificationResults(allResults), nil
}

// 为每个提供者实现GetConfig方法
func (p *DeepseekProvider) GetConfig() (string, string, string) {
	return p.ModelName, p.APIURL, p.APIKey
}

func (p *SiliconFlowProvider) GetConfig() (string, string, string) {
	return p.ModelName, p.APIURL, p.APIKey
}

func (p *AliyunProvider) GetConfig() (string, string, string) {
	return p.ModelName, p.APIURL, p.APIKey
}

func (p *GitHubProvider) GetConfig() (string, string, string) {
	return p.ModelName, p.APIURL, p.APIKey
}

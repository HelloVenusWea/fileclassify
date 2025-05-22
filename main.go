package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileInfo 定义文件信息结构
type FileInfo struct {
	Path     string
	Category string
}

// getFileList 获取指定目录下的所有文件列表
func getFileList(root string) ([]FileInfo, error) {
	var files []FileInfo
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			// 获取相对路径
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, FileInfo{
				Path: relPath,
			})
		}
		return nil
	})
	return files, err
}

func main() {
	// 定义命令行参数
	providerType := flag.String("provider", "", "指定使用的大模型类型 (deepseek, siliconflow, aliyun, github)")
	flag.Parse()

	// 加载配置
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		return
	}

	// 获取指定提供者的配置
	providerConfig, err := config.GetProviderConfig(*providerType)
	if err != nil {
		fmt.Printf("获取模型配置失败: %v\n", err)
		return
	}

	// 创建大模型提供者
	provider, err := NewLLMProvider(*providerType, map[string]string{
		"api_key":    providerConfig.APIKey,
		"api_secret": providerConfig.APISecret,
		"api_url":    providerConfig.APIURL,
		"model_name": providerConfig.ModelName,
	})
	if err != nil {
		fmt.Printf("创建模型提供者失败: %v\n", err)
		return
	}

	// 获取用户输入的文件夹路径
	fmt.Print("请输入要整理的文件夹路径: ")
	reader := bufio.NewReader(os.Stdin)
	folderPath, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("读取输入失败: %v\n", err)
		return
	}
	folderPath = strings.TrimSpace(folderPath)

	// 获取文件列表
	files, err := getFileList(folderPath)
	if err != nil {
		fmt.Printf("获取文件列表失败: %v\n", err)
		return
	}

	fmt.Printf("找到 %d 个文件\n", len(files))

	// 使用大模型对文件进行分类
	fmt.Println("正在使用模型进行分类...")
	classifiedFiles, err := provider.ClassifyFiles(files)
	if err != nil {
		fmt.Printf("分类失败: %v\n", err)
		return
	}

	fmt.Printf("分类完成，共 %d 个分类\n", len(classifiedFiles))
	for category, files := range classifiedFiles {
		fmt.Printf("- %s: %d 个文件\n", category, len(files))
	}

	// 创建分类目录并移动文件
	fmt.Println("\n开始移动文件...")
	for category, files := range classifiedFiles {
		categoryPath := filepath.Join(folderPath, category)
		if err := os.MkdirAll(categoryPath, 0755); err != nil {
			fmt.Printf("创建分类目录失败: %v\n", err)
			continue
		}

		for _, file := range files {
			srcPath := filepath.Join(folderPath, file.Path)
			dstPath := filepath.Join(categoryPath, filepath.Base(file.Path))

			// 检查源文件是否存在
			if _, err := os.Stat(srcPath); os.IsNotExist(err) {
				fmt.Printf("源文件不存在: %s\n", srcPath)
				continue
			}

			// 检查目标文件是否已存在
			if _, err := os.Stat(dstPath); err == nil {
				// 如果目标文件已存在，添加数字后缀
				ext := filepath.Ext(dstPath)
				base := strings.TrimSuffix(dstPath, ext)
				counter := 1
				for {
					newDstPath := fmt.Sprintf("%s_%d%s", base, counter, ext)
					if _, err := os.Stat(newDstPath); os.IsNotExist(err) {
						dstPath = newDstPath
						break
					}
					counter++
				}
			}

			// 使用Copy+Remove替代Rename
			if err := copyFile(srcPath, dstPath); err != nil {
				fmt.Printf("复制文件失败 %s: %v\n", file.Path, err)
				continue
			}
			if err := os.Remove(srcPath); err != nil {
				fmt.Printf("删除源文件失败 %s: %v\n", file.Path, err)
				continue
			}
			fmt.Printf("成功移动文件: %s -> %s\n", file.Path, filepath.Base(dstPath))
		}
	}

	fmt.Println("文件整理完成！")
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

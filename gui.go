package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// MyMainWindow 定义主窗口结构
type MyMainWindow struct {
	window            fyne.Window
	folderPathEdit    *widget.Entry
	providerComboBox  *widget.Select
	recursiveCheckBox *widget.Check
	startButton       *widget.Button
}

// 创建主窗口
func createMainWindow() {
	fmt.Println("开始创建主窗口...")

	// 创建应用
	a := app.NewWithID("com.fileclean.app")
	w := a.NewWindow("智能文件整理程序")
	w.Resize(fyne.NewSize(500, 400))

	// 创建文件夹选择部分
	folderEntry := widget.NewEntry()
	folderEntry.SetPlaceHolder("选择要整理的文件夹...")
	folderEntry.Disable()

	browseButton := widget.NewButton("浏览...", func() {
		dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			if uri != nil {
				folderEntry.SetText(uri.Path())
			}
		}, w)
	})

	// 创建模型选择部分
	providerSelect := widget.NewSelect([]string{"deepseek", "siliconflow", "aliyun", "github"}, nil)
	providerSelect.SetSelected("deepseek")

	// 创建复选框
	recursiveCheck := widget.NewCheck("不递归处理子目录", func(checked bool) {
		treatDirsAsFiles = checked
	})
	recursiveCheck.SetChecked(false)

	// 创建开始按钮
	var startBtn *widget.Button
	startBtn = widget.NewButton("开始整理", func() {
		if folderEntry.Text == "" {
			dialog.ShowError(fmt.Errorf("请先选择要整理的文件夹"), w)
			return
		}

		// 禁用控件
		startBtn.Disable()
		folderEntry.Disable()
		providerSelect.Disable()
		recursiveCheck.Disable()
		browseButton.Disable()

		// 设置全局变量
		providerType = providerSelect.Selected
		treatDirsAsFiles = recursiveCheck.Checked

		// 在新协程中执行文件整理
		go func() {
			defer func() {
				// 在主线程中恢复控件状态
				fyne.Do(func() {
					startBtn.Enable()
					folderEntry.Enable()
					providerSelect.Enable()
					recursiveCheck.Enable()
					browseButton.Enable()
					w.Canvas().Refresh(startBtn)
					w.Canvas().Refresh(folderEntry)
					w.Canvas().Refresh(providerSelect)
					w.Canvas().Refresh(recursiveCheck)
					w.Canvas().Refresh(browseButton)
				})
			}()

			// 加载配置
			config, err := LoadConfig()
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("加载配置失败: %v", err), w)
				})
				return
			}

			// 获取指定提供者的配置
			providerConfig, err := config.GetProviderConfig(providerType)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("获取模型配置失败: %v", err), w)
				})
				return
			}

			// 创建大模型提供者
			provider, err := NewLLMProvider(providerType, map[string]string{
				"api_key":    providerConfig.APIKey,
				"api_secret": providerConfig.APISecret,
				"api_url":    providerConfig.APIURL,
				"model_name": providerConfig.ModelName,
				"timeout":    "120", // 设置120秒超时
			})
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("创建模型提供者失败: %v", err), w)
				})
				return
			}

			// 获取文件列表
			files, err := getFileList(folderEntry.Text)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("获取文件列表失败: %v", err), w)
				})
				return
			}

			// 使用大模型对文件进行分类
			classifiedFiles, err := provider.ClassifyFiles(files)
			if err != nil {
				fyne.Do(func() {
					dialog.ShowError(fmt.Errorf("分类失败: %v", err), w)
				})
				return
			}

			// 创建分类目录并移动文件
			for category, files := range classifiedFiles {
				categoryPath := filepath.Join(folderEntry.Text, category)
				if err := os.MkdirAll(categoryPath, 0755); err != nil {
					fyne.Do(func() {
						dialog.ShowError(fmt.Errorf("创建分类目录失败: %v", err), w)
					})
					continue
				}

				for _, file := range files {
					srcPath := filepath.Join(folderEntry.Text, file.Path)
					dstPath := filepath.Join(categoryPath, filepath.Base(file.Path))

					// 检查源文件是否存在
					if _, err := os.Stat(srcPath); os.IsNotExist(err) {
						continue
					}

					// 如果是目录
					if file.IsDir {
						// 直接移动整个目录
						if err := os.Rename(srcPath, dstPath); err != nil {
							// 如果移动失败，尝试复制后删除
							if err := copyDir(srcPath, dstPath); err != nil {
								fyne.Do(func() {
									dialog.ShowError(fmt.Errorf("移动目录失败: %v", err), w)
								})
								continue
							}
							if err := os.RemoveAll(srcPath); err != nil {
								fyne.Do(func() {
									dialog.ShowError(fmt.Errorf("删除原目录失败: %v", err), w)
								})
							}
						}
					} else {
						// 处理普通文件
						// 检查目标文件是否已存在
						if _, err := os.Stat(dstPath); err == nil {
							// 如果目标文件已存在，添加数字后缀
							ext := filepath.Ext(dstPath)
							base := filepath.Base(dstPath[:len(dstPath)-len(ext)])
							counter := 1
							for {
								newDstPath := filepath.Join(categoryPath, fmt.Sprintf("%s_%d%s", base, counter, ext))
								if _, err := os.Stat(newDstPath); os.IsNotExist(err) {
									dstPath = newDstPath
									break
								}
								counter++
							}
						}

						// 使用Copy+Remove替代Rename
						if err := copyFile(srcPath, dstPath); err != nil {
							fyne.Do(func() {
								dialog.ShowError(fmt.Errorf("复制文件失败: %v", err), w)
							})
							continue
						}
						if err := os.Remove(srcPath); err != nil {
							fyne.Do(func() {
								dialog.ShowError(fmt.Errorf("删除原文件失败: %v", err), w)
							})
						}
					}
				}
			}

			// 删除空文件夹
			for {
				emptyDirs, err := findEmptyDirs(folderEntry.Text)
				if err != nil {
					break
				}

				if len(emptyDirs) == 0 {
					break
				}

				for _, dir := range emptyDirs {
					os.Remove(dir)
				}
			}

			// 显示完成消息
			fyne.Do(func() {
				dialog.ShowInformation("完成", "文件整理完成！", w)
			})
		}()
	})

	// 创建分组
	folderGroup := widget.NewCard("文件夹选择", "", container.NewBorder(nil, nil, nil, browseButton, folderEntry))
	modelGroup := widget.NewCard("模型设置", "", container.NewVBox(
		widget.NewLabel("选择大模型提供者："),
		providerSelect,
		recursiveCheck,
	))
	actionGroup := widget.NewCard("操作", "", container.NewCenter(startBtn))

	// 创建主布局
	content := container.NewVBox(
		widget.NewLabel("智能文件整理程序"),
		widget.NewSeparator(),
		folderGroup,
		modelGroup,
		actionGroup,
	)

	// 添加边距
	paddedContent := container.NewPadded(content)

	w.SetContent(paddedContent)
	w.CenterOnScreen()
	w.ShowAndRun()
}

// copyDir 复制整个目录
func copyDir(src, dst string) error {
	// 创建目标目录
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// 读取源目录
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// 遍历源目录中的所有文件和子目录
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// 递归复制子目录
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// 复制文件
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

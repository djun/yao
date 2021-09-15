package global

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/yaoapp/gou"
	"github.com/yaoapp/kun/exception"
)

// Script 脚本文件类型
type Script struct {
	Name    string
	Type    string
	Content []byte
	File    string
}

// Load 根据配置加载 API, FLow, Model, Plugin
func Load(cfg Config) {
	LoadEngine(cfg.Path)
	LoadApp(cfg.RootAPI, cfg.RootFLow, cfg.RootModel, cfg.RootPlugin)
}

// LoadEngine 加载引擎的 API, Flow, Model 配置
func LoadEngine(from string) {

	var scripts []Script
	if strings.HasPrefix(from, "fs://") || !strings.Contains(from, "://") {
		root := strings.TrimPrefix(from, "fs://")
		scripts = getFilesFS(root, ".json")

		// 监听 flows (这里应该重构)
		go Watch(filepath.Join(root, "flows"), func(op string, file string) {

			if !strings.HasSuffix(file, ".json") {
				return
			}

			if strings.HasSuffix(file, ".js") {
				basName := getFileBaseName(root, file)
				file = basName + ".flow.json"
			}

			if op == "write" || op == "create" {
				script := getFile(root, file)
				gou.LoadFlow(string(script.Content), "xiang."+script.Name) // Reload
				log.Printf("Flow %s 已重新加载完毕", "xiang."+script.Name)
			} else if op == "remove" || op == "rename" {
				name := "xiang." + getFileName(root, file)
				if _, has := gou.Flows[name]; has {
					delete(gou.Flows, name)
					log.Printf("Flow %s 已经移除", name)
				}
			}
		})

		// 监听 models
		go Watch(filepath.Join(root, "models"), func(op string, file string) {

			if !strings.HasSuffix(file, ".json") {
				return
			}
			if op == "write" || op == "create" {
				script := getFile(root, file)
				gou.LoadModel(string(script.Content), "xiang."+script.Name) // Reload
				log.Printf("Model %s 已重新加载完毕", "xiang."+script.Name)
			} else if op == "remove" || op == "rename" {
				name := "xiang." + getFileName(root, file)
				if _, has := gou.Models[name]; has {
					delete(gou.Models, name)
					log.Printf("Model %s 已经移除", name)
				}
			}
		})

		// 监听 apis
		go Watch(filepath.Join(root, "apis"), func(op string, file string) {

			if !strings.HasSuffix(file, ".json") {
				return
			}
			if op == "write" || op == "create" {
				script := getFile(root, file)
				gou.LoadAPI(string(script.Content), "xiang."+script.Name) // Reload
				log.Printf("API %s 已重新加载完毕", "xiang."+script.Name)
			} else if op == "remove" || op == "rename" {
				name := "xiang." + getFileName(root, file)
				if _, has := gou.APIs[name]; has {
					delete(gou.APIs, name)
					log.Printf("API %s 已经移除", name)
				}
			}

			// 重启服务器
			if op == "write" || op == "create" || op == "remove" || op == "rename" {
				ServiceStop(func() {
					log.Printf("服务器重启完毕")
					go ServiceStart()
				})
			}
		})

	} else if strings.HasPrefix(from, "bin://") {
		root := strings.TrimPrefix(from, "bin://")
		scripts = getFilesBin(root, ".json")
	}

	if scripts == nil {
		exception.New("读取文件失败", 500, from).Throw()
	}

	if len(scripts) == 0 {
		exception.New("读取文件失败, 未找到任何可执行脚本", 500, from).Throw()
	}

	// 加载 API, Flow, Models
	for _, script := range scripts {
		switch script.Type {
		case "models":
			gou.LoadModel(string(script.Content), "xiang."+script.Name)
			break
		case "flows":
			gou.LoadFlow(string(script.Content), "xiang."+script.Name)
			break
		case "apis":
			gou.LoadAPI(string(script.Content), "xiang."+script.Name)
			break
		}
	}
}

// LoadApp 加载应用的 API, Flow, Model 和 Plugin
func LoadApp(api string, flow string, model string, plugin string) {

	// 加载API
	if strings.HasPrefix(api, "fs://") || !strings.Contains(api, "://") {
		root := strings.TrimPrefix(api, "fs://")
		scripts := getAppFilesFS(root, ".json")
		for _, script := range scripts {
			// 验证API 加载逻辑
			gou.LoadAPI(string(script.Content), script.Name)
		}

		// 监听API修改
		if Conf.Mode == "debug" {
			go Watch(root, func(op string, file string) {

				if !strings.HasSuffix(file, ".json") {
					return
				}

				if op == "write" || op == "create" {
					script := getAppFile(root, file)
					gou.LoadAPI(string(script.Content), script.Name) // Reload
					log.Printf("API %s 已重新加载完毕", script.Name)

				} else if op == "remove" || op == "rename" {
					name := getAppFileName(root, file)
					if _, has := gou.APIs[name]; has {
						delete(gou.APIs, name)
						log.Printf("API %s 已经移除", name)
					}
				}

				// 重启服务器
				if op == "write" || op == "create" || op == "remove" || op == "rename" {
					ServiceStop(func() {
						log.Printf("服务器重启完毕")
						go ServiceStart()
					})
				}
			})
		}
	}

	// 加载Flow
	if strings.HasPrefix(flow, "fs://") || !strings.Contains(flow, "://") {
		root := strings.TrimPrefix(flow, "fs://")
		scripts := getAppFilesFS(root, ".json")
		for _, script := range scripts {
			gou.LoadFlow(string(script.Content), script.Name)
		}

		// 监听Flow修改
		if Conf.Mode == "debug" {
			go Watch(root, func(op string, file string) {

				if !strings.HasSuffix(file, ".json") && !strings.HasSuffix(file, ".js") {
					return
				}

				if strings.HasSuffix(file, ".js") {
					basName := getAppFileBaseName(root, file)
					file = basName + ".flow.json"
				}

				if op == "write" || op == "create" {
					script := getAppFile(root, file)
					gou.LoadFlow(string(script.Content), script.Name) // Reload
					log.Printf("Flow %s 已重新加载完毕", script.Name)
				} else if op == "remove" || op == "rename" {
					name := getAppFileName(root, file)
					if _, has := gou.Flows[name]; has {
						delete(gou.Flows, name)
						log.Printf("Flow %s 已经移除", name)
					}
				}

			})
		}
	}

	// 加载Model
	if strings.HasPrefix(model, "fs://") || !strings.Contains(model, "://") {
		root := strings.TrimPrefix(model, "fs://")
		scripts := getAppFilesFS(root, ".json")
		for _, script := range scripts {
			gou.LoadModel(string(script.Content), script.Name)
		}

		// 监听Model修改
		if Conf.Mode == "debug" {
			go Watch(root, func(op string, file string) {

				if !strings.HasSuffix(file, ".json") {
					return
				}

				if op == "write" || op == "create" {
					script := getAppFile(root, file)
					gou.LoadModel(string(script.Content), script.Name) // Reload
					log.Printf("Model %s 已重新加载完毕", script.Name)
				} else if op == "remove" || op == "rename" {
					name := getAppFileName(root, file)
					if _, has := gou.Models[name]; has {
						delete(gou.Models, name)
						log.Printf("Model %s 已经移除", name)
					}
				}

			})
		}
	}

	// 加载Plugin
	if strings.HasPrefix(plugin, "fs://") || !strings.Contains(plugin, "://") {
		root := strings.TrimPrefix(plugin, "fs://")
		scripts := getAppPlugins(root, ".so")
		for _, script := range scripts {
			gou.LoadPlugin(script.File, script.Name)
		}

		// 监听Plugin修改
		if Conf.Mode == "debug" {
			go Watch(root, func(op string, file string) {

				if !strings.HasSuffix(file, ".so") {
					return
				}

				if op == "write" || op == "create" {
					script := getAppPluginFile(root, file)
					gou.LoadPlugin(script.File, script.Name) // Reload
					log.Printf("Plugin %s 已重新加载完毕", script.Name)
				} else if op == "remove" || op == "rename" {
					name := getAppPluginFileName(root, file)
					if _, has := gou.Plugins[name]; has {
						delete(gou.Plugins, name)
						log.Printf("Plugin %s 已经移除", name)
					}
				}

			})
		}
	}
}

// / getAppPluins 遍历应用目录，读取文件列表
func getAppPlugins(root string, typ string) []Script {
	files := []Script{}
	root = path.Join(root, "/")
	filepath.Walk(root, func(file string, info os.FileInfo, err error) error {
		if err != nil {
			exception.Err(err, 500).Throw()
			return err
		}
		if strings.HasSuffix(file, typ) {
			files = append(files, getAppPluginFile(root, file))
		}
		return nil
	})
	return files
}

// getAppPluginFile 读取文件
func getAppPluginFile(root string, file string) Script {
	name := getAppPluginFileName(root, file)
	return Script{
		Name: name,
		Type: "plugin",
		File: file,
	}
}

// getAppFile 读取文件
func getAppPluginFileName(root string, file string) string {
	filename := strings.TrimPrefix(file, root+"/")
	namer := strings.Split(filename, ".")
	nametypes := strings.Split(namer[0], "/")
	name := strings.Join(nametypes, ".")
	return name
}

// getAppFilesFS 遍历应用目录，读取文件列表
func getAppFilesFS(root string, typ string) []Script {
	files := []Script{}
	root = path.Join(root, "/")
	filepath.Walk(root, func(filepath string, info os.FileInfo, err error) error {
		if err != nil {
			exception.Err(err, 500).Throw()
			return err
		}
		if strings.HasSuffix(filepath, typ) {
			files = append(files, getAppFile(root, filepath))
		}

		return nil
	})
	return files
}

// getAppFile 读取文件
func getAppFile(root string, filepath string) Script {
	name := getAppFileName(root, filepath)
	file, err := os.Open(filepath)
	if err != nil {
		exception.Err(err, 500).Throw()
	}

	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		exception.Err(err, 500).Throw()
	}
	return Script{
		Name:    name,
		Type:    "app",
		Content: content,
	}
}

// getAppFile 读取文件
func getAppFileName(root string, file string) string {
	filename := strings.TrimPrefix(file, root+"/")
	namer := strings.Split(filename, ".")
	nametypes := strings.Split(namer[0], "/")
	name := strings.Join(nametypes, ".")
	return name
}

// getAppFileBaseName 读取文件base
func getAppFileBaseName(root string, file string) string {
	filename := strings.TrimPrefix(file, root+"/")
	namer := strings.Split(filename, ".")
	return filepath.Join(root, namer[0])
}

// getFilesFS 遍历目录，读取文件列表
func getFilesFS(root string, typ string) []Script {
	files := []Script{}
	root = path.Join(root, "/")
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			exception.Err(err, 500).Throw()
			return err
		}
		if strings.HasSuffix(path, typ) {
			files = append(files, getFile(root, path))
		}
		return nil
	})
	return files
}

// getFile 读取文件
func getFile(root string, path string) Script {
	filename := strings.TrimPrefix(path, root+"/")
	name, typ := getTypeName(filename)
	file, err := os.Open(path)
	if err != nil {
		exception.Err(err, 500).Throw()
	}

	defer file.Close()
	content, err := ioutil.ReadAll(file)
	if err != nil {
		exception.Err(err, 500).Throw()
	}
	return Script{
		Name:    name,
		Type:    typ,
		Content: content,
	}
}

// getFileName 读取文件
func getFileName(root string, file string) string {
	filename := strings.TrimPrefix(file, root+"/")
	name, _ := getTypeName(filename)
	return name
}

// getFileBaseName 读取文件base
func getFileBaseName(root string, file string) string {
	filename := strings.TrimPrefix(file, root+"/")
	namer := strings.Split(filename, ".")
	return filepath.Join(root, namer[0])
}

// getFilesBin 从 bindata 中读取文件列表
func getFilesBin(root string, typ string) []Script {
	files := []Script{}
	binfiles := AssetNames()
	for _, path := range binfiles {
		if strings.HasSuffix(path, typ) {
			file := strings.TrimPrefix(path, root+"/")
			name, typ := getTypeName(file)
			content, err := Asset(path)
			if err != nil {
				exception.Err(err, 500).Throw()
			}
			files = append(files, Script{
				Name:    name,
				Type:    typ,
				Content: content,
			})
		}
	}
	return files
}

func getTypeName(path string) (name string, typ string) {
	namer := strings.Split(path, ".")
	nametypes := strings.Split(namer[0], "/")
	name = strings.Join(nametypes[1:], ".")
	typ = nametypes[0]
	return name, typ
}
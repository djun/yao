package app

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"
	"github.com/yaoapp/gou"
	"github.com/yaoapp/gou/session"
	"github.com/yaoapp/kun/exception"
	"github.com/yaoapp/yao/config"
	"github.com/yaoapp/yao/data"
	"github.com/yaoapp/yao/i18n"
	"github.com/yaoapp/yao/share"
	"github.com/yaoapp/yao/widgets/login"
)

//
// API:
//   GET /api/__yao/app/setting 	-> Default process: yao.app.Xgen
//   GET /api/__yao/app/menu  		-> Default process: yao.app.Menu
//
// Process:
// 	 yao.app.Setting Return the App DSL
// 	 yao.app.Xgen Return the Xgen setting ( merge app & login )
//   yao.app.Menu Return the menu list
//

// Setting the application setting
var Setting *DSL

// LoadAndExport load app
func LoadAndExport(cfg config.Config) error {
	err := Load(cfg)
	if err != nil {
		return err
	}
	return Export()
}

// Load the app DSL
func Load(cfg config.Config) error {

	file := filepath.Join(cfg.Root, "app.json")
	file, err := filepath.Abs(file)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	dsl := &DSL{Optional: OptionalDSL{}, Lang: cfg.Lang}
	err = jsoniter.Unmarshal(data, dsl)
	if err != nil {
		return err
	}

	// Replace Admin Root
	err = dsl.replaceAdminRoot()
	if err != nil {
		return err
	}

	// Load icons
	dsl.icons(cfg)

	Setting = dsl
	return nil
}

// exportAPI export login api
func exportAPI() error {

	if Setting == nil {
		return fmt.Errorf("the app does not init")
	}

	http := gou.HTTP{
		Name:        "Widget App API",
		Description: "Widget App API",
		Version:     share.VERSION,
		Guard:       "bearer-jwt",
		Group:       "__yao/app",
		Paths:       []gou.Path{},
	}

	process := "yao.app.Xgen"
	if Setting.Optional.Setting != "" {
		process = Setting.Optional.Setting
	}

	path := gou.Path{
		Label:       "App Setting",
		Description: "App Setting",
		Guard:       "-",
		Path:        "/setting",
		Method:      "GET",
		Process:     process,
		In:          []string{},
		Out:         gou.Out{Status: 200, Type: "application/json"},
	}
	http.Paths = append(http.Paths, path)

	// POST
	path = gou.Path{
		Label:       "App Setting",
		Description: "App Setting",
		Guard:       "-",
		Path:        "/setting",
		Method:      "POST",
		Process:     process,
		In:          []string{":payload"},
		Out:         gou.Out{Status: 200, Type: "application/json"},
	}
	http.Paths = append(http.Paths, path)

	process = "yao.app.Menu"
	args := []string{}
	if Setting.Menu.Process != "" {
		if Setting.Menu.Args != nil {
			args = Setting.Menu.Args
		}
	}

	path = gou.Path{
		Label:       "App Menu",
		Description: "App Menu",
		Path:        "/menu",
		Method:      "GET",
		Process:     process,
		In:          args,
		Out:         gou.Out{Status: 200, Type: "application/json"},
	}
	http.Paths = append(http.Paths, path)

	process = "yao.app.Icons"
	path = gou.Path{
		Label:       "App Icons",
		Description: "App Icons",
		Path:        "/icons/:name",
		Guard:       "-",
		Method:      "GET",
		Process:     process,
		In:          []string{"$param.name"},
		Out:         gou.Out{Status: 200},
	}
	http.Paths = append(http.Paths, path)

	// api source
	source, err := jsoniter.Marshal(http)
	if err != nil {
		return err
	}

	// load apis
	_, err = gou.LoadAPIReturn(string(source), "widgets.app")
	return err
}

// Export export login api
func Export() error {
	exportProcess()
	return exportAPI()
}

func exportProcess() {
	gou.RegisterProcessHandler("yao.app.setting", processSetting)
	gou.RegisterProcessHandler("yao.app.xgen", processXgen)
	gou.RegisterProcessHandler("yao.app.menu", processMenu)
	gou.RegisterProcessHandler("yao.app.icons", processIcons)
}

func processIcons(process *gou.Process) interface{} {
	process.ValidateArgNums(1)
	name := process.ArgsString(0)
	file, err := filepath.Abs(filepath.Join(config.Conf.Root, "icons", name))
	if err != nil {
		exception.New(err.Error(), 400).Throw()
	}
	content, err := ioutil.ReadFile(file)
	if err != nil {
		exception.New(err.Error(), 400).Throw()
	}
	return string(content)
}

func processMenu(process *gou.Process) interface{} {

	if Setting.Menu.Process != "" {
		return gou.
			NewProcess(Setting.Menu.Process, process.Args...).
			WithGlobal(process.Global).
			WithSID(process.Sid).
			Run()
	}

	args := map[string]interface{}{
		"select": []string{"id", "name", "icon", "parent", "path", "blocks", "visible_menu"},
		"withs": map[string]interface{}{
			"children": map[string]interface{}{
				"query": map[string]interface{}{
					"select": []string{"id", "name", "icon", "parent", "path", "blocks", "visible_menu"},
				},
			},
		},
		"wheres": []map[string]interface{}{
			{"column": "status", "value": "enabled"},
			{"column": "parent", "op": "null"},
		},
		"limit":  200,
		"orders": []map[string]interface{}{{"column": "rank", "option": "asc"}},
	}
	return gou.
		NewProcess("models.xiang.menu.get", args).
		WithGlobal(process.Global).
		WithSID(process.Sid).
		Run()
}

func processSetting(process *gou.Process) interface{} {
	if Setting == nil {
		exception.New("the app does not init", 500).Throw()
		return nil
	}

	sid := process.Sid
	if sid == "" {
		sid = session.ID()
	}

	// Set User ENV
	if process.NumOfArgs() > 0 {
		payload := process.ArgsMap(0, map[string]interface{}{
			"now":  time.Now().Unix(),
			"lang": "en-us",
			"sid":  "",
		})

		if payload["sid"] != "" {
			sid = payload["sid"].(string)
		}

		lang := strings.ToLower(fmt.Sprintf("%v", payload["lang"]))
		session.Global().ID(sid).Set("__yao_lang", lang)
	}

	setting, err := i18n.Trans(process.Lang(), "app", "app", Setting)
	if err != nil {
		exception.New(err.Error(), 500).Throw()
	}

	setting.(*DSL).Sid = sid
	return *setting.(*DSL)
}

func processXgen(process *gou.Process) interface{} {

	if Setting == nil {
		exception.New("the app does not init", 500).Throw()
	}

	sid := process.Sid
	if sid == "" {
		sid = session.ID()
	}

	// Set User ENV
	if process.NumOfArgs() > 0 {
		payload := process.ArgsMap(0, map[string]interface{}{
			"now":  time.Now().Unix(),
			"lang": "en-us",
			"sid":  "",
		})

		if payload["sid"] != "" {
			sid = payload["sid"].(string)
		}

		lang := strings.ToLower(fmt.Sprintf("%v", payload["lang"]))
		session.Global().ID(sid).Set("__yao_lang", lang)
	}

	mode := os.Getenv("YAO_ENV")
	if mode == "" {
		mode = "production"
	}

	xgenLogin := map[string]map[string]interface{}{
		"entry": {"admin": "/x/Welcome"},
	}

	if admin, has := login.Logins["admin"]; has {
		layout := map[string]interface{}{}
		if admin.Layout.Site != "" {
			layout["site"] = admin.Layout.Site
		}

		if admin.Layout.Slogan != "" {
			layout["slogan"] = admin.Layout.Slogan
		}

		if admin.Layout.Cover != "" {
			layout["cover"] = admin.Layout.Cover
		}

		// Translate
		newLayout, err := i18n.Trans(process.Lang(), "login", "admin", layout)
		if err != nil {
			layout = newLayout.(map[string]interface{})
		}

		xgenLogin["entry"]["admin"] = admin.Layout.Entry
		xgenLogin["admin"] = map[string]interface{}{
			"captcha": "/api/__yao/login/admin/captcha?type=digit",
			"login":   "/api/__yao/login/admin",
			"layout":  layout,
		}

	}

	if user, has := login.Logins["user"]; has {
		layout := map[string]interface{}{}
		if user.Layout.Site != "" {
			layout["site"] = user.Layout.Site
		}

		if user.Layout.Slogan != "" {
			layout["slogan"] = user.Layout.Slogan
		}

		if user.Layout.Cover != "" {
			layout["cover"] = user.Layout.Cover
		}

		// Translate
		newLayout, err := i18n.Trans(process.Lang(), "login", "user", layout)
		if err != nil {
			layout = newLayout.(map[string]interface{})
		}

		xgenLogin["entry"]["user"] = user.Layout.Entry
		xgenLogin["user"] = map[string]interface{}{
			"captcha": "/api/__yao/login/user/captcha?type=digit",
			"login":   "/api/__yao/login/user",
			"layout":  layout,
		}
		xgenLogin["layout"] = layout
	}

	xgenSetting := map[string]interface{}{
		"name":        Setting.Name,
		"description": Setting.Description,
		"theme":       Setting.Theme,
		"lang":        Setting.Lang,
		"mode":        mode,
		"apiPrefix":   "__yao",
		"token":       "localStorage",
		"optional":    Setting.Optional,
		"login":       xgenLogin,
	}

	if Setting.Logo != "" {
		xgenSetting["logo"] = Setting.Logo
	}

	if Setting.Favicon != "" {
		xgenSetting["favicon"] = Setting.Favicon
	}

	setting, err := i18n.Trans(process.Lang(), "app", "app", xgenSetting)
	if err != nil {
		exception.New(err.Error(), 500).Throw()
	}

	setting.(map[string]interface{})["sid"] = sid
	return setting.(map[string]interface{})
}

// replaceAdminRoot
func (dsl *DSL) replaceAdminRoot() error {

	if dsl.Optional.AdminRoot == "" {
		dsl.Optional.AdminRoot = "yao"
	}

	root := strings.TrimPrefix(dsl.Optional.AdminRoot, "/")
	root = strings.TrimSuffix(root, "/")
	err := data.ReplaceXGen("/__yao_admin_root/", fmt.Sprintf("/%s/", root))
	if err != nil {
		return err
	}

	return data.ReplaceXGen("\"__yao_admin_root\"", fmt.Sprintf("\"%s\"", root))
}

// icons
func (dsl *DSL) icons(cfg config.Config) {

	favicon := filepath.Join(cfg.Root, "icons", "app.ico")
	if _, err := os.Stat(favicon); err == nil {
		dsl.Favicon = fmt.Sprintf("/api/__yao/app/icons/app.ico")
	}

	logo := filepath.Join(cfg.Root, "icons", "app.png")
	if _, err := os.Stat(logo); err == nil {
		dsl.Logo = fmt.Sprintf("/api/__yao/app/icons/app.png")
	}
}

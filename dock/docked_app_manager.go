/**
 * Copyright (C) 2014 Deepin Technology Co., Ltd.
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 **/

package dock

import (
	"gir/gio-2.0"
	"gir/glib-2.0"
	"io/ioutil"
	"os"
	"path/filepath"
	"pkg.deepin.io/lib/dbus"
)

const (
	dockSchema           string = "com.deepin.dde.dock"
	settingKeyDockedApps string = "docked-apps"
	dockedItemTemplate   string = `[Desktop Entry]
Name={{ .Name }}
Exec={{ .Exec }}
Icon={{ .Icon }}
Type=Application
Terminal=false
StartupNotify=false
`
)

// DockedAppManager是管理已驻留程序的管理器。
type DockedAppManager struct {
	settings      *gio.Settings
	dockManager   *DockManager
	dockedAppList []string

	// Docked是信号，在某程序驻留成功后被触发，并将该程序的id发送给信号的接受者。
	Docked func(id string) // find indicator on front-end.
	// Undocked是信号，在某已驻留程序被移除驻留后被触发，将被移除程序id发送给信号接受者。
	Undocked func(id string)
}

func NewDockedAppManager(dockManager *DockManager) *DockedAppManager {
	m := &DockedAppManager{
		dockManager: dockManager,
	}
	m.init()
	return m
}

func (m *DockedAppManager) init() {
	m.settings = gio.NewSettings(dockSchema)
	if m.settings == nil {
		return
	}
	m.handleOldConfigFile()

	m.dockedAppList = uniqStrSlice(m.settings.GetStrv(settingKeyDockedApps))
	m.saveAppList(m.dockedAppList)
}

func (m *DockedAppManager) destroy() {
	if m.settings != nil {
		m.settings.Unref()
	}
	dbus.UnInstallObject(m)
}

func (m *DockedAppManager) handleOldConfigFile() {
	conf := glib.NewKeyFile()
	defer conf.Free()

	confFile := filepath.Join(glib.GetUserConfigDir(), "dock/apps.ini")
	_, err := conf.LoadFromFile(confFile, glib.KeyFileFlagsNone)
	if err != nil {
		logger.Debug("Open old dock config file failed:", err)
		return
	}

	inited, err := conf.GetBoolean("__Config__", "inited")
	if err == nil && inited {
		return
	}

	_, ids, err := conf.GetStringList("__Config__", "Position")
	if err != nil {
		logger.Debug("Read docked app from old config file failed:", err)
		return
	}
	ids = uniqStrSlice(ids)
	for _, id := range ids {
		if a := NewAppInfo(id); a != nil {
			a.Destroy()
			continue
		}

		exec, _ := conf.GetString(id, "CmdLine")
		icon, _ := conf.GetString(id, "Icon")
		title, _ := conf.GetString(id, "Name")
		createScratchDesktopFile(id, title, icon, exec)
	}

	m.saveAppList(ids)
	conf.SetBoolean("__Config__", "inited", true)

	_, content, err := conf.ToData()
	if err != nil {
		return
	}

	var mode os.FileMode = 0666
	stat, err := os.Lstat(confFile)
	if err == nil {
		mode = stat.Mode()
	}

	err = ioutil.WriteFile(confFile, []byte(content), mode)
	if err != nil {
		logger.Warning("Save Config file failed:", err)
	}
}

// DockedAppList返回程序id列表。
func (m *DockedAppManager) DockedAppList() []string {
	return m.dockedAppList
}

// IsDocked通过传入的程序id判断一个程序是否已经驻留。
func (m *DockedAppManager) IsDocked(appId string) bool {
	return isStrInSlice(appId, m.dockedAppList)
}

type dockedItemInfo struct {
	Name, Icon, Exec string
}

func (m *DockedAppManager) dockAppEntry(entry *AppEntry) bool {
	if entry.appInfo == nil {
		logger.Warning("dockAppEntry failed, entry.appInfo is nil")
		return false
	}

	m.saveDockedAppList()
	appId := entry.appInfo.GetId()
	m.emitSignal("Docked", appId)
	return true
}

func (m *DockedAppManager) undockAppEntry(appId string) bool {
	m.saveDockedAppList()
	m.emitSignal("Undocked", appId)
	return true
}

// RequestDock驻留程序。通常情况下只需要传递程序id即可，在特殊情况下需要传入title，icon以及cmd。
// title表示前端程序的tooltip内容，icon为程序图标，cmd为程序的启动命令。
// 成功后会触发Docked信号。
func (m *DockedAppManager) RequestDock(id, title, icon, cmd string) bool {
	return m.dockManager.requestDock(id, title, icon, cmd)
}

// RequestUndock 通过程序id移除已驻留程序。成功后会触发Undocked信号。
func (m *DockedAppManager) RequestUndock(id string) bool {
	return m.dockManager.undockEntryByAppId(id)
}

func (m *DockedAppManager) emitSignal(name string, values ...interface{}) {
	logger.Debugf("Emit Signal %v %v", name, values)
	dbus.Emit(m, name, values...)
}

// 保存 apps 到 gsettings
func (m *DockedAppManager) saveAppList(apps []string) {
	m.settings.SetStrv(settingKeyDockedApps, apps)
	gio.SettingsSync()
}

func (m *DockedAppManager) saveDockedAppList() {
	apps := m.dockManager.getDockedAppList()
	if !strSliceEqual(m.dockedAppList, apps) {
		logger.Debugf("Save gsettings %s: %#v", settingKeyDockedApps, apps)
		m.saveAppList(apps)
		m.dockedAppList = apps
	}
}

// 废弃，请使用新接口RequestDock
func (m *DockedAppManager) Dock(id, title, icon, cmd string) bool {
	return m.RequestDock(id, title, icon, cmd)
}

// 废弃，请使用新接口RequestUndock
func (m *DockedAppManager) Undock(id string) bool {
	return m.RequestUndock(id)
}

// TODO: 删除此函数，因为拼写错误
func (m *DockedAppManager) ReqeustDock(id, title, icon, cmd string) bool {
	return m.RequestDock(id, title, icon, cmd)
}

// Sort 废弃
func (m *DockedAppManager) Sort([]string) {
}

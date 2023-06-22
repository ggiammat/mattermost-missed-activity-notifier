package main

import (
	"reflect"

	"github.com/pkg/errors"
)

type configuration struct {
	RunInterval                            int
	IgnoreMessagesNewerThan                int
	ResetLastNotificationTimestamp         bool
	DryRun                                 bool
	NotifyOnlyNewMessagesFromStartup       bool
	KeepStatusHistoryInterval              int
	RunStatsToKeep                         int
	EmailSubTitle                          string
	EmailButtonText                        string
	EmailFooterLine1                       string
	EmailFooterLine2                       string
	EmailFooterLine3                       string
	DebugLogEnabled                        bool
	UserDefaultPrefEnabled                 bool
	UserDefaultPrefNotifyNotFollowed       bool
	UserDefaultPrefCountNotFollowed        bool
	UserDefaultPrefCountMM                 bool
	UserDefaultPrefCountPreviouslyNotified bool
	UserDefaultIncludeSystemMessages       bool
	UserDefaultPrefIncludeMessagesFromBots bool
	DebugHTTPToken                         string
}

// Clone shallow copies the configuration. Your implementation may require a deep copy if
// your configuration has reference types.
func (c *configuration) Clone() *configuration {
	var clone = *c
	return &clone
}

// getConfiguration retrieves the active configuration under lock, making it safe to use
// concurrently. The active configuration may change underneath the client of this method, but
// the struct returned by this API call is considered immutable.
func (p *MANPlugin) getConfiguration() *configuration {
	p.configurationLock.RLock()
	defer p.configurationLock.RUnlock()

	if p.configuration == nil {
		return &configuration{}
	}

	return p.configuration
}

// setConfiguration replaces the active configuration under lock.
//
// Do not call setConfiguration while holding the configurationLock, as sync.Mutex is not
// reentrant. In particular, avoid using the plugin API entirely, as this may in turn trigger a
// hook back into the plugin. If that hook attempts to acquire this lock, a deadlock may occur.
//
// This method panics if setConfiguration is called with the existing configuration. This almost
// certainly means that the configuration was modified without being cloned and may result in
// an unsafe access.
func (p *MANPlugin) setConfiguration(configuration *configuration) {
	p.configurationLock.Lock()
	defer p.configurationLock.Unlock()

	if configuration != nil && p.configuration == configuration {
		// Ignore assignment if the configuration struct is empty. Go will optimize the
		// allocation for same to point at the same memory address, breaking the check
		// above.
		if reflect.ValueOf(*configuration).NumField() == 0 {
			return
		}

		panic("setConfiguration called with the existing configuration")
	}

	p.configuration = configuration
}

// OnConfigurationChange is invoked when configuration changes may have been made.
func (p *MANPlugin) OnConfigurationChange() error {
	var configuration = new(configuration)

	// Load the public configuration fields from the Mattermost server configuration.
	if err := p.API.LoadPluginConfiguration(configuration); err != nil {
		return errors.Wrap(err, "failed to load plugin configuration")
	}

	restartMANJob := p.configuration != nil && (p.configuration.RunInterval != configuration.RunInterval)

	p.setConfiguration(configuration)

	// recreate backend since configuration changes affects its fields
	errB := p.CreateMattermostBackend()
	if errB != nil {
		p.backend.LogError("error creating backend: %s", errB)
	}

	if restartMANJob {
		p.backend.LogInfo("MAN run interval changed in configuration. Restarting scheduler")
		errD := p.deactivateMANJob()
		if errD != nil {
			p.backend.LogError("error deactivating MANJob: %s", errD)
		}
		errA := p.activateMANJob()
		if errA != nil {
			p.backend.LogError("error activating MANJob: %s", errA)
		}
	}
	return nil
}

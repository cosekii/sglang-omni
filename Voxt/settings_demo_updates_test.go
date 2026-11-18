package main

func settingsDemoUpdateCases() []settingsDemoCase {
	return []settingsDemoCase{
		{name: "read current announcement", fixture: "en/settings-about-whats-new", steps: historySteps("Read"), want: []string{"showAnnouncement"}},
		{name: "check updates", fixture: "en/settings-about-updates", steps: historySteps("Check for Updates…"), want: []string{"checkForUpdates"}},
		{name: "restart after update", fixture: "en/settings-about-updates-check-only", steps: historySteps("Restart Voxt"), want: []string{"relaunch"}},
		{name: "automatic update checks", fixture: "en/settings-about-updates", steps: historySteps("Check for updates automatically"), want: []string{"setAutoCheck false"}},
		{name: "beta updates", fixture: "en/settings-about-updates", steps: historySteps("Get beta versions"), want: []string{"setBeta true"}},
		{name: "automatic update installs", fixture: "en/settings-about-updates", steps: historySteps("Install updates without asking"), want: []string{"setAutoInstall true"}},
	}
}

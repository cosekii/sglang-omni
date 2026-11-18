package main

type updatesActions interface {
	showAnnouncement()
	checkForUpdates()
	setAutoCheck(on bool)
	setAutoInstall(on bool)
	setBeta(on bool)
	relaunch()
}

type releaseActions interface {
	checkReleasesNow()
	downloadRelease(address string)
}

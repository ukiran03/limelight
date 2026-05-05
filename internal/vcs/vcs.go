package vcs

import "runtime/debug"

func Version() string {
	binfo, ok := debug.ReadBuildInfo()
	if ok {
		return binfo.Main.Version
	}
	return ""
}

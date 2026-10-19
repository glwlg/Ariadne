package plugins

import "ariadne/internal/contracts"

func executeAirPlay(query string) []contracts.SearchResult {
	return []contracts.SearchResult{
		toolWindowResult("airplay", "打开 iPhone 投屏", "iPhone 投屏", "连接 iPhone，查看投屏状态并调整播放设置。", "open_airplay"),
	}
}

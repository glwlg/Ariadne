//go:build windows

package filesearch

import "testing"

func TestDefaultFileSearchFilterExcludesWindowsDriveRuntimeNoise(t *testing.T) {
	t.Setenv("APPDATA", `C:\Users\luwei\AppData\Roaming`)
	t.Setenv("LOCALAPPDATA", `C:\Users\luwei\AppData\Local`)
	t.Setenv("TEMP", `C:\Users\luwei\AppData\Local\Temp`)
	t.Setenv("TMP", `C:\Users\luwei\AppData\Local\Temp`)
	t.Setenv("WINDIR", `C:\Windows`)
	t.Setenv("ProgramData", `C:\ProgramData`)
	filter := newFileSearchFilter(DefaultFileSearchPolicy())

	excluded := []string{
		`C:\Users\luwei\AppData\Local\D3DSCache\4921e6f431b0944a\F4EB2D6C-ED2B-4BDD-AD9D-F913287E6768.dxcache-shm`,
		`C:\Users\luwei\AppData\Local\Temp\edge-local-state.tmp`,
		`C:\Users\luwei\AppData\Roaming\fancun.exe\EBWebView\Default\Code Cache\js\index-dir\temp-index\the-real-index`,
		`C:\Users\luwei\AppData\Local\Google\Chrome\User Data\Default\Service Worker\CacheStorage\2fb1\index-dir\the-real-index`,
		`C:\Users\luwei\AppData\Roaming\Mozilla\Firefox\Profiles\profile.default-release\cache2\entries\A1B2C3`,
		`C:\Users\luwei\AppData\Local\Packages\MicrosoftWindows.Client.CBS_cw5n1h2txyewy\LocalState\EBWebView\Default\Code Cache\js\index-dir\56a2206c.tmp`,
		`C:\Users\luwei\AppData\Roaming\Tencent\wechat\radium\web\profiles\default\Network\TransportSecurity\transportsecurity~rf18504f85.tmp`,
		`C:\Users\luwei\AppData\Roaming\utForpc\24772217\MjQ3NzIyMTc=\UTForPC.db-journal`,
		`C:\Users\luwei\AppData\Roaming\Tencent\xwechat\net\kvcomm\key_2961520020_4065598005_1_1783301876_7742_3600_ready.statistic`,
		`C:\Users\luwei\AppData\Local\Microsoft\OneDrive\logs\ListSync\Consumer_9cf4ddf2cc2d756a\NucleusLocal-2026-07-06.0001.57016.27.odlgz`,
		`C:\Users\luwei\AppData\Local\Microsoft\Edge Beta\User Data\Default\Network\TransportSecurity`,
		`C:\Users\luwei\AppData\Roaming\Fancun\capture_images\capture-20260706-094138.970034400-535c8e18a2ac.png`,
		`C:\Users\luwei\.gemini\cache\state.json`,
		`C:\Users\luwei\.m2\repository\org\example\artifact\1.0.0\artifact-1.0.0.jar`,
		`C:\Windows\System32\ntdll.dll`,
		`C:\pagefile.sys`,
		`P:\App\Telegram Desktop\tdata\user_data`,
		`P:\AppData\wechat\xwechat_files\wxid_abc\FileStorage\Cache\temp.dat`,
		`P:\DevApp\Unity\Editor\Data\PlaybackEngines\AndroidPlayer\Tools\gradle\cache.bin`,
		`P:\Program Files\Tencent\WeChat\WeChatAppEx.exe`,
		`P:\workspace\env\npm\_cacache\content-v2\sha512\aa\bb`,
		`P:\workspace\glwlg\app\Fancun\.venv\Lib\site-packages\pkg\__init__.py`,
		`P:\workspace\glwlg\app\Fancun\.git\refs\codex\turn-diffs\captures\1783301386046\base`,
		`P:\workspace\glwlg\app\Fancun\frontend\node_modules\.pnpm\vue\index.js`,
	}
	for _, path := range excluded {
		if !filter.Excludes(path) {
			t.Fatalf("default filter should exclude runtime noise path %q", path)
		}
	}

	if filter.Excludes(`P:\workspace\glwlg\app\Fancun\internal\filesearch\policy.go`) {
		t.Fatal("default filter should not exclude ordinary source files")
	}
}

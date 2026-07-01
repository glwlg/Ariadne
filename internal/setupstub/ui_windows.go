//go:build windows

package setupstub

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	installerWindowWidth  = 760
	installerWindowHeight = 580

	installerIDBack      = 1001
	installerIDNext      = 1002
	installerIDCancel    = 1003
	installerIDBrowse    = 1004
	installerIDPath      = 1005
	installerIDStartMenu = 1006
	installerIDDesktop   = 1007
	installerIDAutostart = 1008
	installerIDLaunch    = 1009
	installerIDAgreement = 1010
	installerIDIndexSvc  = 1011

	installerWMClose           = 0x0010
	installerWMDestroy         = 0x0002
	installerWMCommand         = 0x0111
	installerWMSetFont         = 0x0030
	installerWMSetIcon         = 0x0080
	installerWMColorStatic     = 0x0138
	installerWMColorButton     = 0x0135
	installerSTMSetIcon        = 0x0170
	installerBMGetCheck        = 0x00F0
	installerBMSetCheck        = 0x00F1
	installerBSTChecked        = 1
	installerIconSmall         = 0
	installerIconBig           = 1
	installerImageIcon         = 1
	installerTransparentBkMode = 1

	installerWSOverlapped   = 0x00000000
	installerWSCaption      = 0x00C00000
	installerWSSysMenu      = 0x00080000
	installerWSMinimizeBox  = 0x00020000
	installerWSChild        = 0x40000000
	installerWSVisible      = 0x10000000
	installerWSTabStop      = 0x00010000
	installerWSBorder       = 0x00800000
	installerWSVScroll      = 0x00200000
	installerESAutoHScroll  = 0x0080
	installerESMultiline    = 0x0004
	installerESAutoVScroll  = 0x0040
	installerESReadOnly     = 0x0800
	installerBSAutoCheckbox = 0x00000003
	installerBSDefPush      = 0x00000001
	installerSSLeft         = 0x00000000
	installerSSIcon         = 0x00000003
	installerSSCenterImage  = 0x00000200

	installerSWHide          = 0
	installerSWShow          = 5
	installerCWUseDefault    = int32(-2147483648)
	installerIDCursorArrow   = 32512
	installerIDIApplication  = 32512
	installerBIFReturnFSDirs = 0x00000001
	installerBIFNewUI        = 0x00000040
	installerCoInitApartment = 0x2
	installerCoInitOK        = 0
	installerCoInitSFalse    = 1
)

const (
	colorWhite       uint32 = 0x00ffffff
	colorPanel       uint32 = 0x00fafafa
	colorBrandAccent uint32 = 0x00d4a017
	colorText        uint32 = 0x00321f0f
	colorMuted       uint32 = 0x00706a5f
)

type installerPoint struct {
	X int32
	Y int32
}

type installerMsg struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   installerPoint
}

type installerWndClassEx struct {
	Size        uint32
	Style       uint32
	WndProc     uintptr
	ClassExtra  int32
	WindowExtra int32
	Instance    windows.Handle
	Icon        windows.Handle
	Cursor      windows.Handle
	Background  windows.Handle
	MenuName    *uint16
	ClassName   *uint16
	SmallIcon   windows.Handle
}

type installerBrowseInfo struct {
	Owner       windows.Handle
	Root        uintptr
	DisplayName *uint16
	Title       *uint16
	Flags       uint32
	Callback    uintptr
	Param       uintptr
	Image       int32
}

type installerLogFont struct {
	Height         int32
	Width          int32
	Escapement     int32
	Orientation    int32
	Weight         int32
	Italic         byte
	Underline      byte
	StrikeOut      byte
	CharSet        byte
	OutPrecision   byte
	ClipPrecision  byte
	Quality        byte
	PitchAndFamily byte
	FaceName       [32]uint16
}

type interactiveInstallSelection struct {
	InstallDir               string
	CreateStartMenuShortcut  bool
	CreateDesktopShortcut    bool
	InstallFileSearchService bool
	AutoStart                bool
	LaunchAfterInstall       bool
}

func (selection interactiveInstallSelection) args() []string {
	args := []string{"-InstallDir", selection.InstallDir}
	if !selection.CreateStartMenuShortcut {
		args = append(args, "--no-start-menu-shortcut")
	}
	if !selection.CreateDesktopShortcut {
		args = append(args, "--no-desktop-shortcut")
	}
	if selection.AutoStart {
		args = append(args, "--autostart")
	}
	if selection.InstallFileSearchService {
		args = append(args, "--install-file-search-service")
	} else {
		args = append(args, "--no-file-search-service")
	}
	if selection.LaunchAfterInstall {
		args = append(args, "--launch-after-install")
	}
	return args
}

type installerWindow struct {
	productName string
	version     string

	hwnd     windows.Handle
	instance windows.Handle

	defaultFont uintptr
	titleFont   uintptr
	headingFont uintptr
	smallFont   uintptr

	whiteBrush windows.Handle
	panelBrush windows.Handle

	pageControls   []windows.Handle
	controlText    map[windows.Handle]uint32
	controlBrushes map[windows.Handle]windows.Handle

	page       int
	draft      interactiveInstallSelection
	selected   interactiveInstallSelection
	cancelled  bool
	err        error
	pathEdit   windows.Handle
	startMenu  windows.Handle
	desktop    windows.Handle
	autostart  windows.Handle
	launch     windows.Handle
	indexSvc   windows.Handle
	agreement  windows.Handle
	backButton windows.Handle
	nextButton windows.Handle
}

var activeInstallerWindow *installerWindow

var (
	kernel32SetupStubUI = windows.NewLazySystemDLL("kernel32.dll")
	gdi32SetupStubUI    = windows.NewLazySystemDLL("gdi32.dll")
	ole32SetupStubUI    = windows.NewLazySystemDLL("ole32.dll")

	procGetModuleHandleSetupStubUI     = kernel32SetupStubUI.NewProc("GetModuleHandleW")
	procRegisterClassExSetupStubUI     = user32SetupStub.NewProc("RegisterClassExW")
	procCreateWindowExSetupStubUI      = user32SetupStub.NewProc("CreateWindowExW")
	procDefWindowProcSetupStubUI       = user32SetupStub.NewProc("DefWindowProcW")
	procShowWindowSetupStubUI          = user32SetupStub.NewProc("ShowWindow")
	procUpdateWindowSetupStubUI        = user32SetupStub.NewProc("UpdateWindow")
	procGetMessageSetupStubUI          = user32SetupStub.NewProc("GetMessageW")
	procTranslateMessageSetupStubUI    = user32SetupStub.NewProc("TranslateMessage")
	procDispatchMessageSetupStubUI     = user32SetupStub.NewProc("DispatchMessageW")
	procPostQuitMessageSetupStubUI     = user32SetupStub.NewProc("PostQuitMessage")
	procDestroyWindowSetupStubUI       = user32SetupStub.NewProc("DestroyWindow")
	procSendMessageSetupStubUI         = user32SetupStub.NewProc("SendMessageW")
	procLoadCursorSetupStubUI          = user32SetupStub.NewProc("LoadCursorW")
	procLoadImageSetupStubUI           = user32SetupStub.NewProc("LoadImageW")
	procLoadIconSetupStubUI            = user32SetupStub.NewProc("LoadIconW")
	procGetSystemMetricsSetupStubUI    = user32SetupStub.NewProc("GetSystemMetrics")
	procSetWindowTextSetupStubUI       = user32SetupStub.NewProc("SetWindowTextW")
	procGetWindowTextSetupStubUI       = user32SetupStub.NewProc("GetWindowTextW")
	procGetWindowTextLenSetupStubUI    = user32SetupStub.NewProc("GetWindowTextLengthW")
	procIsDialogMessageSetupStubUI     = user32SetupStub.NewProc("IsDialogMessageW")
	procSetFocusSetupStubUI            = user32SetupStub.NewProc("SetFocus")
	procEnableWindowSetupStubUI        = user32SetupStub.NewProc("EnableWindow")
	procSetBkModeSetupStubUI           = gdi32SetupStubUI.NewProc("SetBkMode")
	procSetTextColorSetupStubUI        = gdi32SetupStubUI.NewProc("SetTextColor")
	procCreateSolidBrushSetupStubUI    = gdi32SetupStubUI.NewProc("CreateSolidBrush")
	procCreateFontIndirectSetupStubUI  = gdi32SetupStubUI.NewProc("CreateFontIndirectW")
	procCoInitializeExSetupStubUI      = ole32SetupStubUI.NewProc("CoInitializeEx")
	procCoUninitializeSetupStubUI      = ole32SetupStubUI.NewProc("CoUninitialize")
	procCoTaskMemFreeSetupStubUI       = ole32SetupStubUI.NewProc("CoTaskMemFree")
	procSHBrowseForFolderSetupStubUI   = shell32SetupStub.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDListSetupStubUI = shell32SetupStub.NewProc("SHGetPathFromIDListW")

	installerWindowProcCallback = windows.NewCallback(installerWindowProc)
)

func RunInteractive(payload []byte, options Options) (Result, error) {
	if options.ProductName == "" {
		options.ProductName = "Ariadne"
	}
	if options.Version == "" {
		options.Version = "dev"
	}

	selection, ok, err := showInstallerWindow(options)
	if err != nil {
		return Result{}, err
	}
	if !ok {
		return Result{Action: ActionCancelled}, nil
	}

	options.Args = selection.args()
	return Run(payload, options)
}

func showInstallerWindow(options Options) (interactiveInstallSelection, bool, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	comInitialized := initializeInstallerCOM()
	if comInitialized {
		defer procCoUninitializeSetupStubUI.Call()
	}

	window := &installerWindow{
		productName: options.ProductName,
		version:     options.Version,
		draft: interactiveInstallSelection{
			InstallDir:               defaultInstallDir(options.ProductName),
			CreateStartMenuShortcut:  true,
			CreateDesktopShortcut:    true,
			InstallFileSearchService: false,
			LaunchAfterInstall:       true,
		},
		cancelled:      true,
		controlText:    map[windows.Handle]uint32{},
		controlBrushes: map[windows.Handle]windows.Handle{},
	}
	activeInstallerWindow = window
	defer func() {
		activeInstallerWindow = nil
	}()

	if err := window.create(); err != nil {
		return interactiveInstallSelection{}, false, err
	}

	var msg installerMsg
	for {
		ret, _, err := procGetMessageSetupStubUI.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 {
			return interactiveInstallSelection{}, false, installerWin32Error("read installer message", err)
		}
		if ret == 0 {
			break
		}
		if window.hwnd != 0 {
			handled, _, _ := procIsDialogMessageSetupStubUI.Call(uintptr(window.hwnd), uintptr(unsafe.Pointer(&msg)))
			if handled != 0 {
				continue
			}
		}
		procTranslateMessageSetupStubUI.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageSetupStubUI.Call(uintptr(unsafe.Pointer(&msg)))
	}

	if window.err != nil {
		return interactiveInstallSelection{}, false, window.err
	}
	if window.cancelled {
		return interactiveInstallSelection{}, false, nil
	}
	return window.selected, true, nil
}

func (window *installerWindow) create() error {
	instance, _, err := procGetModuleHandleSetupStubUI.Call(0)
	if instance == 0 {
		return installerWin32Error("get installer module handle", err)
	}
	window.instance = windows.Handle(instance)
	window.initTheme()

	className, err := windows.UTF16PtrFromString("AriadneSetupWizardWindow")
	if err != nil {
		return err
	}
	cursor, _, _ := procLoadCursorSetupStubUI.Call(0, installerIDCursorArrow)
	bigIcon := loadInstallerIcon(window.instance, 48)
	smallIcon := loadInstallerIcon(window.instance, 16)
	wndClass := installerWndClassEx{
		Size:       uint32(unsafe.Sizeof(installerWndClassEx{})),
		WndProc:    installerWindowProcCallback,
		Instance:   window.instance,
		Icon:       bigIcon,
		Cursor:     windows.Handle(cursor),
		Background: window.whiteBrush,
		ClassName:  className,
		SmallIcon:  smallIcon,
	}
	if ret, _, registerErr := procRegisterClassExSetupStubUI.Call(uintptr(unsafe.Pointer(&wndClass))); ret == 0 && !isClassAlreadyRegistered(registerErr) {
		return installerWin32Error("register installer window", registerErr)
	}

	title, err := windows.UTF16PtrFromString(window.productName + " 安装向导")
	if err != nil {
		return err
	}
	screenWidth, _, _ := procGetSystemMetricsSetupStubUI.Call(0)
	screenHeight, _, _ := procGetSystemMetricsSetupStubUI.Call(1)
	x := installerCWUseDefault
	y := installerCWUseDefault
	if screenWidth > installerWindowWidth && screenHeight > installerWindowHeight {
		x = int32((int(screenWidth) - installerWindowWidth) / 2)
		y = int32((int(screenHeight) - installerWindowHeight) / 2)
	}
	hwnd, _, createErr := procCreateWindowExSetupStubUI.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(title)),
		uintptr(installerWSOverlapped|installerWSCaption|installerWSSysMenu|installerWSMinimizeBox),
		uintptr(x),
		uintptr(y),
		installerWindowWidth,
		installerWindowHeight,
		0,
		0,
		uintptr(window.instance),
		0,
	)
	if hwnd == 0 {
		return installerWin32Error("create installer window", createErr)
	}
	window.hwnd = windows.Handle(hwnd)
	procSendMessageSetupStubUI.Call(hwnd, installerWMSetIcon, installerIconBig, uintptr(bigIcon))
	procSendMessageSetupStubUI.Call(hwnd, installerWMSetIcon, installerIconSmall, uintptr(smallIcon))

	window.render()
	procShowWindowSetupStubUI.Call(hwnd, installerSWShow)
	procUpdateWindowSetupStubUI.Call(hwnd)
	return nil
}

func (window *installerWindow) initTheme() {
	window.whiteBrush = createBrush(colorWhite)
	window.panelBrush = createBrush(colorPanel)
	window.defaultFont = createFont("Segoe UI", -13, 400)
	window.smallFont = createFont("Segoe UI", -12, 400)
	window.headingFont = createFont("Segoe UI Semibold", -17, 600)
	window.titleFont = createFont("Segoe UI Semibold", -28, 700)
}

func (window *installerWindow) render() {
	window.destroyPageControls()
	window.renderRail()
	switch window.page {
	case 0:
		window.renderWelcomePage()
	case 1:
		window.renderAgreementPage()
	case 2:
		window.renderOptionsPage()
	default:
		window.renderReadyPage()
	}
	window.renderFooter()
	procUpdateWindowSetupStubUI.Call(uintptr(window.hwnd))
}

func (window *installerWindow) renderRail() {
	window.addText("ARIADNE", 34, 36, 110, 20, window.smallFont, colorMuted, window.whiteBrush)
	window.addText(window.productName, 34, 58, 150, 26, window.headingFont, colorText, window.whiteBrush)
	window.addText("本地优先 · 普通启动", 34, 92, 168, 22, window.smallFont, colorMuted, window.whiteBrush)
	window.addPanel(220, 30, 1, 430, window.panelBrush)

	steps := []string{"欢迎", "用户协议", "安装选项", "确认安装"}
	for index, label := range steps {
		y := int32(150 + index*46)
		prefix := fmt.Sprintf("%d", index+1)
		color := colorMuted
		if window.page == index {
			color = colorText
			prefix = "●"
		}
		window.addText(prefix, 34, y, 24, 22, window.headingFont, color, window.whiteBrush)
		window.addText(label, 68, y+1, 120, 22, window.defaultFont, color, window.whiteBrush)
	}
	window.addText("安装服务时会请求管理员权限。", 34, 438, 168, 42, window.smallFont, colorMuted, window.whiteBrush)
}

func (window *installerWindow) renderWelcomePage() {
	window.addText("欢迎安装 Ariadne", 254, 42, 430, 38, window.titleFont, colorText, window.whiteBrush)
	window.addText("把启动器、文件搜索、截图历史、剪贴板历史、工作记忆和桌面工具中心放到统一入口。", 256, 92, 430, 40, window.defaultFont, colorMuted, window.whiteBrush)

	window.addPanel(256, 152, 430, 1, window.panelBrush)
	window.addText("安装内容", 256, 180, 120, 24, window.headingFont, colorText, window.whiteBrush)
	window.addText("• Ariadne 桌面应用与卸载入口", 260, 218, 410, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText("• 内置 Ariadne 文件索引", 260, 248, 410, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText("• 当前用户的开始菜单和桌面快捷方式", 260, 278, 410, 22, window.defaultFont, colorText, window.whiteBrush)

	window.addText("默认安装到", 256, 330, 110, 22, window.smallFont, colorMuted, window.whiteBrush)
	window.addText(window.draft.InstallDir, 256, 354, 430, 22, window.defaultFont, colorText, window.whiteBrush)
}

func (window *installerWindow) renderAgreementPage() {
	window.addText("用户协议", 254, 42, 430, 38, window.titleFont, colorText, window.whiteBrush)
	window.addText("继续安装前，请阅读并接受 Ariadne 的使用条款。", 256, 92, 430, 28, window.defaultFont, colorMuted, window.whiteBrush)

	agreement := strings.Join([]string{
		"Ariadne 用户协议",
		"",
		"1. Ariadne 是本地优先的 Windows 桌面效率工具。应用数据默认保存在当前用户目录。",
		"2. 截图、剪贴板、工作记忆、OCR、AI 和导出能力受应用内隐私设置与用户确认控制。",
		"3. 文件索引服务用于维护本机文件搜索索引；桌面应用仍以当前用户权限运行。",
		"4. 使用外部 AI、远程接口或第三方服务时，请确认相关账号、数据和网络策略符合你的使用场景。",
		"5. 除已明确授权内容外，复制、分发或二次开发请先取得维护者许可。",
		"6. 继续安装表示你理解并接受以上条款。",
	}, "\r\n")
	window.addEdit(256, 134, 430, 218, agreement, installerESMultiline|installerESReadOnly|installerESAutoVScroll|installerWSVScroll, 0)
	window.agreement = window.addCheckbox(258, 370, 390, 24, "我已阅读并同意用户协议", false, installerIDAgreement)
}

func (window *installerWindow) renderOptionsPage() {
	window.addText("安装选项", 254, 42, 430, 38, window.titleFont, colorText, window.whiteBrush)
	window.addText("选择安装位置、搜索服务和启动方式。", 256, 92, 430, 28, window.defaultFont, colorMuted, window.whiteBrush)

	window.addText("安装位置", 256, 140, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.pathEdit = window.addEdit(256, 168, 346, 30, window.draft.InstallDir, installerESAutoHScroll, installerIDPath)
	window.addButton(614, 167, 74, 32, "浏览...", installerIDBrowse, false)

	window.addText("快捷方式", 256, 224, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.startMenu = window.addCheckbox(258, 256, 300, 24, "创建开始菜单入口", window.draft.CreateStartMenuShortcut, installerIDStartMenu)
	window.desktop = window.addCheckbox(258, 286, 300, 24, "创建桌面快捷方式", window.draft.CreateDesktopShortcut, installerIDDesktop)

	window.addText("搜索服务", 256, 326, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.indexSvc = window.addCheckbox(258, 356, 390, 24, "安装文件索引服务", window.draft.InstallFileSearchService, installerIDIndexSvc)

	window.addText("启动方式", 256, 396, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.autostart = window.addCheckbox(258, 426, 190, 24, "随 Windows 启动", window.draft.AutoStart, installerIDAutostart)
	window.launch = window.addCheckbox(456, 426, 230, 24, "安装完成后启动 Ariadne", window.draft.LaunchAfterInstall, installerIDLaunch)
}

func (window *installerWindow) renderReadyPage() {
	window.addText("准备安装", 254, 42, 430, 38, window.titleFont, colorText, window.whiteBrush)
	window.addText("确认以下设置，点击安装后开始复制文件并创建所选入口。", 256, 92, 430, 32, window.defaultFont, colorMuted, window.whiteBrush)

	window.addText("安装位置", 256, 148, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.addText(window.draft.InstallDir, 256, 178, 430, 24, window.defaultFont, colorText, window.whiteBrush)
	window.addPanel(256, 218, 430, 1, window.panelBrush)

	window.addText("将创建", 256, 246, 120, 22, window.headingFont, colorText, window.whiteBrush)
	window.addText(optionLabel(window.draft.CreateStartMenuShortcut, "开始菜单入口"), 260, 282, 390, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText(optionLabel(window.draft.CreateDesktopShortcut, "桌面快捷方式"), 260, 312, 390, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText(optionLabel(window.draft.InstallFileSearchService, "文件索引服务"), 260, 342, 390, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText(optionLabel(window.draft.AutoStart, "随 Windows 启动"), 260, 372, 390, 22, window.defaultFont, colorText, window.whiteBrush)
	window.addText(optionLabel(window.draft.LaunchAfterInstall, "安装完成后启动"), 260, 402, 390, 22, window.defaultFont, colorText, window.whiteBrush)
}

func (window *installerWindow) renderFooter() {
	window.addPanel(220, 464, installerWindowWidth-220, 1, window.panelBrush)
	window.backButton = window.addButton(410, 486, 86, 32, "上一步", installerIDBack, false)
	window.addButton(508, 486, 86, 32, "取消", installerIDCancel, false)
	nextText := "下一步"
	if window.page == 3 {
		nextText = "安装"
	}
	window.nextButton = window.addButton(606, 486, 96, 32, nextText, installerIDNext, true)
	enable := uintptr(0)
	if window.page > 0 {
		enable = 1
	}
	procEnableWindowSetupStubUI.Call(uintptr(window.backButton), enable)
}

func (window *installerWindow) addPanel(x int32, y int32, width int32, height int32, brush windows.Handle) windows.Handle {
	handle := window.addControl("STATIC", "", installerWSChild|installerWSVisible, x, y, width, height, 0, 0)
	window.setBrush(handle, brush)
	return handle
}

func (window *installerWindow) addText(text string, x int32, y int32, width int32, height int32, font uintptr, color uint32, brush windows.Handle) windows.Handle {
	handle := window.addControl("STATIC", text, installerWSChild|installerWSVisible|installerSSLeft, x, y, width, height, 0, font)
	window.controlText[handle] = color
	window.setBrush(handle, brush)
	return handle
}

func (window *installerWindow) addIcon(x int32, y int32, width int32, height int32, size int32) windows.Handle {
	handle := window.addControl("STATIC", "", installerWSChild|installerWSVisible|installerSSIcon|installerSSCenterImage, x, y, width, height, 0, 0)
	icon := loadInstallerIcon(window.instance, size)
	procSendMessageSetupStubUI.Call(uintptr(handle), installerSTMSetIcon, uintptr(icon), 0)
	return handle
}

func (window *installerWindow) addButton(x int32, y int32, width int32, height int32, text string, id int, defaultButton bool) windows.Handle {
	style := uint32(installerWSChild | installerWSVisible | installerWSTabStop)
	if defaultButton {
		style |= installerBSDefPush
	}
	return window.addControl("BUTTON", text, style, x, y, width, height, id, window.defaultFont)
}

func (window *installerWindow) addCheckbox(x int32, y int32, width int32, height int32, text string, checked bool, id int) windows.Handle {
	handle := window.addControl("BUTTON", text, installerWSChild|installerWSVisible|installerWSTabStop|installerBSAutoCheckbox, x, y, width, height, id, window.defaultFont)
	window.controlText[handle] = colorText
	window.setBrush(handle, window.whiteBrush)
	window.setChecked(handle, checked)
	return handle
}

func (window *installerWindow) addEdit(x int32, y int32, width int32, height int32, text string, extraStyle uint32, id int) windows.Handle {
	return window.addControl("EDIT", text, installerWSChild|installerWSVisible|installerWSTabStop|installerWSBorder|extraStyle, x, y, width, height, id, window.defaultFont)
}

func (window *installerWindow) addControl(className string, text string, style uint32, x int32, y int32, width int32, height int32, id int, font uintptr) windows.Handle {
	classNamePtr, err := windows.UTF16PtrFromString(className)
	if err != nil {
		window.err = err
		return 0
	}
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		window.err = err
		return 0
	}
	hwnd, _, createErr := procCreateWindowExSetupStubUI.Call(
		0,
		uintptr(unsafe.Pointer(classNamePtr)),
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(style),
		uintptr(x),
		uintptr(y),
		uintptr(width),
		uintptr(height),
		uintptr(window.hwnd),
		uintptr(id),
		uintptr(window.instance),
		0,
	)
	if hwnd == 0 {
		window.err = installerWin32Error("create installer control", createErr)
		return 0
	}
	if font == 0 {
		font = window.defaultFont
	}
	if font != 0 {
		procSendMessageSetupStubUI.Call(hwnd, installerWMSetFont, font, 1)
	}
	handle := windows.Handle(hwnd)
	window.pageControls = append(window.pageControls, handle)
	return handle
}

func installerWindowProc(hwnd uintptr, message uint32, wParam uintptr, lParam uintptr) uintptr {
	switch message {
	case installerWMCommand:
		id := int(wParam & 0xffff)
		if activeInstallerWindow != nil {
			switch id {
			case installerIDNext:
				activeInstallerWindow.next()
				return 0
			case installerIDBack:
				activeInstallerWindow.back()
				return 0
			case installerIDCancel:
				activeInstallerWindow.cancel()
				return 0
			case installerIDBrowse:
				activeInstallerWindow.browseInstallDir()
				return 0
			}
		}
	case installerWMColorStatic, installerWMColorButton:
		if activeInstallerWindow != nil {
			return activeInstallerWindow.controlColor(wParam, windows.Handle(lParam))
		}
	case installerWMClose:
		if activeInstallerWindow != nil {
			activeInstallerWindow.cancel()
			return 0
		}
	case installerWMDestroy:
		procPostQuitMessageSetupStubUI.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcSetupStubUI.Call(hwnd, uintptr(message), wParam, lParam)
	return ret
}

func (window *installerWindow) next() {
	switch window.page {
	case 0:
		window.page = 1
	case 1:
		if !window.isChecked(window.agreement) {
			ShowError(window.productName+" 安装", "请先阅读并同意用户协议。")
			return
		}
		window.page = 2
	case 2:
		if !window.saveOptions() {
			return
		}
		window.page = 3
	default:
		window.accept()
		return
	}
	window.render()
}

func (window *installerWindow) back() {
	if window.page == 2 {
		window.saveOptions()
	}
	if window.page > 0 {
		window.page--
		window.render()
	}
}

func (window *installerWindow) saveOptions() bool {
	installDir := strings.TrimSpace(window.windowText(window.pathEdit))
	if installDir == "" {
		ShowError(window.productName+" 安装", "请选择安装位置。")
		return false
	}
	window.draft = interactiveInstallSelection{
		InstallDir:               filepath.Clean(installDir),
		CreateStartMenuShortcut:  window.isChecked(window.startMenu),
		CreateDesktopShortcut:    window.isChecked(window.desktop),
		InstallFileSearchService: window.isChecked(window.indexSvc),
		AutoStart:                window.isChecked(window.autostart),
		LaunchAfterInstall:       window.isChecked(window.launch),
	}
	return true
}

func (window *installerWindow) accept() {
	window.selected = window.draft
	window.cancelled = false
	procDestroyWindowSetupStubUI.Call(uintptr(window.hwnd))
}

func (window *installerWindow) cancel() {
	window.cancelled = true
	procDestroyWindowSetupStubUI.Call(uintptr(window.hwnd))
}

func (window *installerWindow) browseInstallDir() {
	if window.pathEdit == 0 {
		return
	}
	path := browseForFolder(window.hwnd, "选择安装位置")
	if path == "" {
		return
	}
	window.setWindowText(window.pathEdit, path)
}

func (window *installerWindow) controlColor(hdc uintptr, control windows.Handle) uintptr {
	procSetBkModeSetupStubUI.Call(hdc, installerTransparentBkMode)
	if color, ok := window.controlText[control]; ok {
		procSetTextColorSetupStubUI.Call(hdc, uintptr(color))
	}
	if brush, ok := window.controlBrushes[control]; ok && brush != 0 {
		return uintptr(brush)
	}
	return uintptr(window.whiteBrush)
}

func (window *installerWindow) setBrush(handle windows.Handle, brush windows.Handle) {
	if handle != 0 && brush != 0 {
		window.controlBrushes[handle] = brush
	}
}

func (window *installerWindow) destroyPageControls() {
	for _, control := range window.pageControls {
		if control != 0 {
			procDestroyWindowSetupStubUI.Call(uintptr(control))
			delete(window.controlText, control)
			delete(window.controlBrushes, control)
		}
	}
	window.pageControls = nil
	window.pathEdit = 0
	window.startMenu = 0
	window.desktop = 0
	window.autostart = 0
	window.launch = 0
	window.indexSvc = 0
	window.agreement = 0
	window.backButton = 0
	window.nextButton = 0
}

func (window *installerWindow) setChecked(hwnd windows.Handle, checked bool) {
	value := uintptr(0)
	if checked {
		value = installerBSTChecked
	}
	procSendMessageSetupStubUI.Call(uintptr(hwnd), installerBMSetCheck, value, 0)
}

func (window *installerWindow) isChecked(hwnd windows.Handle) bool {
	if hwnd == 0 {
		return false
	}
	value, _, _ := procSendMessageSetupStubUI.Call(uintptr(hwnd), installerBMGetCheck, 0, 0)
	return value == installerBSTChecked
}

func (window *installerWindow) setWindowText(hwnd windows.Handle, text string) {
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procSetWindowTextSetupStubUI.Call(uintptr(hwnd), uintptr(unsafe.Pointer(textPtr)))
}

func (window *installerWindow) windowText(hwnd windows.Handle) string {
	length, _, _ := procGetWindowTextLenSetupStubUI.Call(uintptr(hwnd))
	if length == 0 {
		return ""
	}
	buffer := make([]uint16, int(length)+1)
	procGetWindowTextSetupStubUI.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	return windows.UTF16ToString(buffer)
}

func browseForFolder(owner windows.Handle, title string) string {
	displayName := make([]uint16, windows.MAX_PATH)
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return ""
	}
	info := installerBrowseInfo{
		Owner:       owner,
		DisplayName: &displayName[0],
		Title:       titlePtr,
		Flags:       installerBIFReturnFSDirs | installerBIFNewUI,
	}
	pidl, _, _ := procSHBrowseForFolderSetupStubUI.Call(uintptr(unsafe.Pointer(&info)))
	if pidl == 0 {
		return ""
	}
	defer procCoTaskMemFreeSetupStubUI.Call(pidl)
	buffer := make([]uint16, windows.MAX_PATH)
	ok, _, _ := procSHGetPathFromIDListSetupStubUI.Call(pidl, uintptr(unsafe.Pointer(&buffer[0])))
	if ok == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer)
}

func loadInstallerIcon(instance windows.Handle, size int32) windows.Handle {
	name, _ := windows.UTF16PtrFromString("APP")
	icon, _, _ := procLoadImageSetupStubUI.Call(
		uintptr(instance),
		uintptr(unsafe.Pointer(name)),
		installerImageIcon,
		uintptr(size),
		uintptr(size),
		0,
	)
	if icon != 0 {
		return windows.Handle(icon)
	}
	fallback, _, _ := procLoadIconSetupStubUI.Call(0, installerIDIApplication)
	return windows.Handle(fallback)
}

func createBrush(color uint32) windows.Handle {
	brush, _, _ := procCreateSolidBrushSetupStubUI.Call(uintptr(color))
	return windows.Handle(brush)
}

func createFont(face string, height int32, weight int32) uintptr {
	var font installerLogFont
	font.Height = height
	font.Weight = weight
	font.CharSet = 1
	font.Quality = 5
	copy(font.FaceName[:], windows.StringToUTF16(face))
	handle, _, _ := procCreateFontIndirectSetupStubUI.Call(uintptr(unsafe.Pointer(&font)))
	return handle
}

func optionLabel(enabled bool, label string) string {
	if enabled {
		return "✓ " + label
	}
	return "— " + label
}

func initializeInstallerCOM() bool {
	ret, _, _ := procCoInitializeExSetupStubUI.Call(0, installerCoInitApartment)
	return ret == installerCoInitOK || ret == installerCoInitSFalse
}

func isClassAlreadyRegistered(err error) bool {
	if errno, ok := err.(syscall.Errno); ok {
		return errno == 1410
	}
	return false
}

func installerWin32Error(operation string, err error) error {
	if errno, ok := err.(syscall.Errno); ok && errno == 0 {
		return fmt.Errorf("%s failed", operation)
	}
	if err == nil {
		return fmt.Errorf("%s failed", operation)
	}
	return fmt.Errorf("%s: %w", operation, err)
}

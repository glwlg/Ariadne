#[cfg(windows)]
mod windows_tray {
    use std::{
        mem::size_of,
        os::windows::ffi::OsStrExt,
        ptr::{null, null_mut},
        sync::atomic::{AtomicIsize, Ordering},
        sync::mpsc::{self, SyncSender},
        thread::{self, JoinHandle},
        time::Duration,
    };

    use windows_sys::Win32::{
        Foundation::{HINSTANCE, HWND, LPARAM, LRESULT, POINT, WPARAM},
        System::{LibraryLoader::GetModuleHandleW, Threading::GetCurrentThreadId},
        UI::{
            Shell::{
                NIF_ICON, NIF_MESSAGE, NIF_TIP, NIM_ADD, NIM_DELETE, NIM_SETVERSION,
                NOTIFYICON_VERSION_4, NOTIFYICONDATAW, Shell_NotifyIconW,
            },
            WindowsAndMessaging::{
                AppendMenuW, CreatePopupMenu, CreateWindowExW, DefWindowProcW, DestroyMenu,
                DestroyWindow, DispatchMessageW, GetCursorPos, GetMessageW, IDI_APPLICATION,
                IMAGE_ICON, LR_DEFAULTSIZE, LR_LOADFROMFILE, LoadIconW, LoadImageW, MF_STRING,
                PeekMessageW, PostMessageW, PostQuitMessage, PostThreadMessageW, RegisterClassW,
                RegisterWindowMessageW, SW_HIDE, SW_SHOW, SetForegroundWindow, ShowWindow,
                TPM_BOTTOMALIGN, TPM_LEFTALIGN, TPM_RIGHTBUTTON, TrackPopupMenu, TranslateMessage,
                UnregisterClassW, WM_APP, WM_CLOSE, WM_COMMAND, WM_CONTEXTMENU, WM_DESTROY,
                WM_LBUTTONDBLCLK, WM_LBUTTONUP, WM_NULL, WM_QUIT, WM_RBUTTONUP, WNDCLASSW,
                WS_EX_TOOLWINDOW,
            },
        },
    };

    const TRAY_ID: u32 = 1;
    const TRAY_CALLBACK: u32 = WM_APP + 1;
    const COMMAND_SHOW: usize = 1;
    const COMMAND_EXIT: usize = 2;

    static LAUNCHER_HWND: AtomicIsize = AtomicIsize::new(0);

    pub(crate) fn set_launcher_window(hwnd: HWND) {
        LAUNCHER_HWND.store(hwnd as isize, Ordering::Release);
    }

    pub(crate) fn show_launcher() {
        let launcher = LAUNCHER_HWND.load(Ordering::Acquire) as HWND;
        if launcher.is_null() {
            return;
        }
        unsafe {
            ShowWindow(launcher, SW_SHOW);
            SetForegroundWindow(launcher);
        }
    }

    pub(crate) struct TrayHandle {
        thread_id: u32,
        join: Option<JoinHandle<()>>,
    }

    impl Drop for TrayHandle {
        fn drop(&mut self) {
            unsafe {
                let _ = PostThreadMessageW(self.thread_id, WM_QUIT, 0, 0);
            }
            if let Some(join) = self.join.take() {
                let _ = join.join();
            }
        }
    }

    pub(crate) fn start() -> Option<TrayHandle> {
        let (ready_tx, ready_rx) = mpsc::sync_channel(1);
        let join = thread::spawn(move || run(ready_tx));
        match ready_rx.recv_timeout(Duration::from_secs(2)) {
            Ok(Ok(thread_id)) => Some(TrayHandle {
                thread_id,
                join: Some(join),
            }),
            _ => {
                let _ = join.join();
                None
            }
        }
    }

    fn load_tray_icon() -> windows_sys::Win32::UI::WindowsAndMessaging::HICON {
        let icon_path = std::env::current_exe()
            .ok()
            .and_then(|path| path.parent().map(|parent| parent.join("logo.ico")));
        if let Some(path) = icon_path {
            let path = path
                .as_os_str()
                .encode_wide()
                .chain(std::iter::once(0))
                .collect::<Vec<_>>();
            let icon = unsafe {
                LoadImageW(
                    null_mut(),
                    path.as_ptr(),
                    IMAGE_ICON,
                    0,
                    0,
                    LR_LOADFROMFILE | LR_DEFAULTSIZE,
                ) as windows_sys::Win32::UI::WindowsAndMessaging::HICON
            };
            if !icon.is_null() {
                return icon;
            }
        }
        unsafe { LoadIconW(null_mut(), IDI_APPLICATION) }
    }
    const TASKBAR_CREATED: &[u16] = &[
        84, 97, 115, 107, 98, 97, 114, 67, 114, 101, 97, 116, 101, 100, 0,
    ];

    fn add_tray_icon(icon_data: &mut NOTIFYICONDATAW) -> bool {
        unsafe {
            icon_data.Anonymous.uVersion = 0;
            for attempt in 0..3 {
                if Shell_NotifyIconW(NIM_ADD, icon_data) != 0 {
                    icon_data.Anonymous.uVersion = NOTIFYICON_VERSION_4;
                    let _ = Shell_NotifyIconW(NIM_SETVERSION, icon_data);
                    return true;
                }
                if attempt < 2 {
                    thread::sleep(Duration::from_millis(200));
                }
            }
            false
        }
    }
    fn show_tray_menu(hwnd: HWND) {
        unsafe {
            let menu = CreatePopupMenu();
            if menu.is_null() {
                return;
            }

            let show_label = "显示 Ariadne\0".encode_utf16().collect::<Vec<_>>();
            let exit_label = "退出 Ariadne\0".encode_utf16().collect::<Vec<_>>();
            if AppendMenuW(menu, MF_STRING, COMMAND_SHOW, show_label.as_ptr()) != 0
                && AppendMenuW(menu, MF_STRING, COMMAND_EXIT, exit_label.as_ptr()) != 0
            {
                let mut point: POINT = std::mem::zeroed();
                if GetCursorPos(&mut point) != 0 {
                    SetForegroundWindow(hwnd);
                    TrackPopupMenu(
                        menu,
                        TPM_LEFTALIGN | TPM_BOTTOMALIGN | TPM_RIGHTBUTTON,
                        point.x,
                        point.y,
                        0,
                        hwnd,
                        null(),
                    );
                    let _ = PostMessageW(hwnd, WM_NULL, 0, 0);
                }
            }
            DestroyMenu(menu);
        }
    }

    fn run(ready_tx: SyncSender<Result<u32, ()>>) {
        unsafe {
            let thread_id = GetCurrentThreadId();
            let mut queue_probe = std::mem::zeroed();
            PeekMessageW(&mut queue_probe, null_mut(), 0, 0, 0);

            let instance: HINSTANCE = GetModuleHandleW(null());
            if instance.is_null() {
                ready_tx.send(Err(())).ok();
                return;
            }

            // Keep the class process-specific so a second packaged instance does not
            // fail RegisterClassW before it can add its tray icon.
            let class_name = format!("AriadneTray-{}", std::process::id())
                .encode_utf16()
                .chain(std::iter::once(0))
                .collect::<Vec<_>>();
            let mut class: WNDCLASSW = std::mem::zeroed();
            class.lpfnWndProc = Some(window_proc);
            class.hInstance = instance;
            class.lpszClassName = class_name.as_ptr();
            if RegisterClassW(&class) == 0 {
                ready_tx.send(Err(())).ok();
                return;
            }

            let hwnd = CreateWindowExW(
                WS_EX_TOOLWINDOW,
                class_name.as_ptr(),
                class_name.as_ptr(),
                0,
                0,
                0,
                0,
                0,
                null_mut(),
                null_mut(),
                instance,
                null_mut(),
            );
            if hwnd.is_null() {
                UnregisterClassW(class_name.as_ptr(), instance);
                ready_tx.send(Err(())).ok();
                return;
            }

            ShowWindow(hwnd, SW_HIDE);

            let mut icon_data: NOTIFYICONDATAW = std::mem::zeroed();
            icon_data.cbSize = size_of::<NOTIFYICONDATAW>() as u32;
            icon_data.hWnd = hwnd;
            icon_data.uID = TRAY_ID;
            icon_data.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
            icon_data.uCallbackMessage = TRAY_CALLBACK;
            icon_data.hIcon = load_tray_icon();
            for (slot, value) in icon_data.szTip.iter_mut().zip("Ariadne".encode_utf16()) {
                *slot = value;
            }
            if !add_tray_icon(&mut icon_data) {
                DestroyWindow(hwnd);
                UnregisterClassW(class_name.as_ptr(), instance);
                ready_tx.send(Err(())).ok();
                return;
            }

            if ready_tx.send(Ok(thread_id)).is_err() {
                let _ = Shell_NotifyIconW(NIM_DELETE, &mut icon_data);
                DestroyWindow(hwnd);
                UnregisterClassW(class_name.as_ptr(), instance);
                return;
            }

            let taskbar_created = RegisterWindowMessageW(TASKBAR_CREATED.as_ptr());
            let mut message = std::mem::zeroed();
            while GetMessageW(&mut message, null_mut(), 0, 0) > 0 {
                if taskbar_created != 0 && message.message == taskbar_created {
                    let _ = add_tray_icon(&mut icon_data);
                    continue;
                }
                TranslateMessage(&message);
                DispatchMessageW(&message);
            }

            let _ = Shell_NotifyIconW(NIM_DELETE, &mut icon_data);
            DestroyWindow(hwnd);
            UnregisterClassW(class_name.as_ptr(), instance);
        }
    }

    unsafe extern "system" fn window_proc(
        hwnd: HWND,
        message: u32,
        wparam: WPARAM,
        lparam: LPARAM,
    ) -> LRESULT {
        match message {
            TRAY_CALLBACK => match lparam as u32 {
                WM_LBUTTONUP | WM_LBUTTONDBLCLK => {
                    show_launcher();
                    0
                }
                WM_RBUTTONUP | WM_CONTEXTMENU => {
                    show_tray_menu(hwnd);
                    0
                }
                _ => 0,
            },
            WM_COMMAND => match (wparam as usize) & 0xFFFF {
                COMMAND_SHOW => {
                    show_launcher();
                    0
                }
                COMMAND_EXIT => unsafe {
                    let launcher = LAUNCHER_HWND.load(Ordering::Acquire) as HWND;
                    if !launcher.is_null() {
                        let _ = PostMessageW(launcher, WM_CLOSE, 0, 0);
                    }
                    DestroyWindow(hwnd);
                    0
                },
                _ => 0,
            },
            WM_CLOSE => unsafe {
                DestroyWindow(hwnd);
                0
            },
            WM_DESTROY => unsafe {
                PostQuitMessage(0);
                0
            },
            _ => unsafe { DefWindowProcW(hwnd, message, wparam, lparam) },
        }
    }
}

#[cfg(windows)]
pub(crate) fn set_launcher_window(window: &gpui::Window) {
    use raw_window_handle::{HasWindowHandle, RawWindowHandle};

    let Ok(handle) = HasWindowHandle::window_handle(window) else {
        return;
    };
    let RawWindowHandle::Win32(handle) = handle.as_raw() else {
        return;
    };
    windows_tray::set_launcher_window(handle.hwnd.get() as *mut core::ffi::c_void);
}

#[cfg(windows)]
pub(crate) use windows_tray::start;

#[cfg(windows)]
pub(crate) fn show_launcher() {
    windows_tray::show_launcher();
}

#[cfg(not(windows))]
pub(crate) struct TrayHandle;

#[cfg(not(windows))]
pub(crate) fn start() -> Option<TrayHandle> {
    None
}

#[cfg(not(windows))]
pub(crate) fn set_launcher_window(_window: &gpui::Window) {}

#[cfg(not(windows))]
pub(crate) fn show_launcher() {}

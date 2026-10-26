#include <gst/app/gstappsrc.h>
#include <stdio.h>
#include "ariadne-window.h"

static PVOID volatile video_window;
static HANDLE control_started;
static HWND control_window;
static volatile LONG control_ready = 2, packet_count, reject_press;
static volatile LONG keyboard_ready = 1;
static AriadnePointerPacket packets[64];
static WNDPROC resize_original;
#define TEST_SYSTEM_RESIZE (WM_APP + 301)
#define TEST_FOCUS (WM_APP + 302)

static LRESULT CALLBACK test_resize_proc(HWND window, UINT message, WPARAM wp, LPARAM lp)
{
    if (message == TEST_FOCUS) {
        SetFocus(wp ? window : NULL);
        return GetFocus() == window;
    }
    if (message == TEST_SYSTEM_RESIZE) {
        RECT client;
        GetClientRect(window, &client);
        SendMessageW(window, WM_ENTERSIZEMOVE, 0, 0);
        SetCapture(window);
        if (GetCapture() != window) return -1;
        SendMessageW(window, WM_SIZE, SIZE_RESTORED, MAKELPARAM(client.right, client.bottom));
        int captured = GetCapture() == window;
        SendMessageW(window, WM_MOUSEMOVE, MK_LBUTTON, MAKELPARAM(client.right / 2, client.bottom / 2));
        SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
        ReleaseCapture();
        SendMessageW(window, WM_EXITSIZEMOVE, 0, 0);
        return captured;
    }
    return CallWindowProcW(resize_original, window, message, wp, lp);
}

static gboolean test_focus(HWND window, gboolean focus)
{
    resize_original = (WNDPROC)SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)test_resize_proc);
    LRESULT result = SendMessageW(window, TEST_FOCUS, focus, 0);
    SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)resize_original);
    return !!result == focus;
}

static gboolean test_resize_capture(HWND window)
{
    resize_original = (WNDPROC)SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)test_resize_proc);
    LRESULT result = SendMessageW(window, TEST_SYSTEM_RESIZE, 0, 0);
    SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)resize_original);
    return result == 1;
}

static gboolean window_has_capture(HWND window)
{
    GUITHREADINFO info = {.cbSize = sizeof(info)};
    return GetGUIThreadInfo(GetWindowThreadProcessId(window, NULL), &info) && info.hwndCapture == window;
}

static gboolean test_resize_borders(HWND window)
{
    RECT rect;
    GetWindowRect(window, &rect);
    int cx = (rect.left + rect.right) / 2, cy = (rect.top + rect.bottom) / 2;
    const int points[][3] = {
        {rect.left + 1, cy, HTLEFT}, {rect.right - 2, cy, HTRIGHT},
        {cx, rect.top + 1, HTTOP}, {cx, rect.bottom - 2, HTBOTTOM},
        {rect.left + 1, rect.top + 1, HTTOPLEFT}, {rect.right - 2, rect.top + 1, HTTOPRIGHT},
        {rect.left + 1, rect.bottom - 2, HTBOTTOMLEFT}, {rect.right - 2, rect.bottom - 2, HTBOTTOMRIGHT}
    };
    for (int i = 0; i < G_N_ELEMENTS(points); i++) {
        LRESULT hit = SendMessageW(window, WM_NCHITTEST, 0, MAKELPARAM(points[i][0], points[i][1]));
        if (hit != points[i][2]) {
            fprintf(stderr, "Resize border %d: expected %d, got %lld\n", i, points[i][2], (long long)hit);
            return FALSE;
        }
    }
    return TRUE;
}

static LRESULT CALLBACK test_control_proc(HWND window, UINT message, WPARAM wp, LPARAM lp)
{
    if (message == WM_COPYDATA) {
        COPYDATASTRUCT *data = (COPYDATASTRUCT *)lp;
        if (data->dwData != ARIADNE_POINTER_TAG || data->cbData != sizeof(AriadnePointerPacket)) return 0;
        AriadnePointerPacket *packet = data->lpData;
        if (packet->version != 1) return 0;
        if (packet->kind == POINTER_QUERY) return control_ready;
        if (packet->kind == KEYBOARD_QUERY) return keyboard_ready;
        if (packet->kind == POINTER_DOWN && reject_press) return 0;
        LONG count = InterlockedCompareExchange(&packet_count, 0, 0);
        if (count < 64) {
            packets[count] = *packet;
            InterlockedIncrement(&packet_count);
        }
        return 1;
    }
    if (message == WM_DESTROY) { PostQuitMessage(0); return 0; }
    return DefWindowProcW(window, message, wp, lp);
}

static DWORD WINAPI test_control_thread(void *unused)
{
    WNDCLASSW cls = {.lpfnWndProc = test_control_proc, .lpszClassName = L"Ariadne.ScreenInput.Test"};
    RegisterClassW(&cls);
    control_window = CreateWindowW(cls.lpszClassName, ARIADNE_CONTROL_TITLE, 0, 0, 0, 1, 1,
        NULL, NULL, GetModuleHandleW(NULL), NULL);
    SetEvent(control_started);
    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0) { TranslateMessage(&msg); DispatchMessageW(&msg); }
    return 0;
}

static int test_pointer_mapping(void)
{
    POINT point;
    if (ariadne_map_pointer(1000, 1000, 1920, 1080, 1, 1, 500, 20, FALSE, &point)) return 1;
    if (!ariadne_map_pointer(1000, 1000, 1920, 1080, 1, 1, 500, 500, FALSE, &point) ||
        abs(point.x - 16384) > 64 || abs(point.y - 16384) > 64) return 1;
    if (!ariadne_map_pointer(1000, 1000, 1080, 1920, 1, 1, -200, 2000, TRUE, &point) ||
        point.x != 0 || point.y != 32767) return 1;
    if (ariadne_map_pointer(0, 1000, 1080, 1920, 1, 1, 0, 0, FALSE, &point)) return 1;
    if (!ariadne_map_pointer(1000, 500, 500, 500, 2, 1, 500, 250, FALSE, &point) ||
        abs(point.x - 16384) > 64 || abs(point.y - 16384) > 64) return 1;
    return 0;
}

static int test_key_mapping(void)
{
    // Letters follow physical scan codes even when the desktop layout changes their virtual key.
    if (ariadne_key_usage('Q', 0x1e0001) != 4 ||
        ariadne_key_usage(VK_SHIFT, 0x2a0001) != 225 ||
        ariadne_key_usage(VK_SHIFT, 0x360001) != 229 ||
        ariadne_key_usage(VK_CONTROL, 0x011d0001) != 228 ||
        ariadne_key_usage(VK_MENU, 0x01380001) != 230 ||
        ariadne_key_usage(VK_RETURN, 0x1c0001) != 40 ||
        ariadne_key_usage(VK_RETURN, 0x011c0001) != 88 ||
        ariadne_key_usage(VK_INSERT, 0x520001) != 98 ||
        ariadne_key_usage(VK_INSERT, 0x01520001) != 73 ||
        ariadne_key_usage(VK_OEM_1, 0x270001) != 51 ||
        ariadne_key_usage(VK_F12, 0x580001) != 69 ||
        ariadne_key_usage(VK_PAUSE, 0x450001) != 72 ||
        ariadne_key_usage(VK_VOLUME_UP, 0x01300001) != 0) {
        fprintf(stderr, "Keyboard scan code mapping failed\n");
        return 1;
    }
    return 0;
}

static int test_direct_input(HWND window)
{
    int failed = 0;
#define INPUT_CHECK(condition, description) do { \
    if (!(condition)) { \
        fprintf(stderr, "%s; packets=%ld:", description, packet_count); \
        for (int pi = 0; pi < packet_count && pi < 64; pi++) fprintf(stderr, " %d(%d,%d)", packets[pi].kind, packets[pi].x, packets[pi].y); \
        fprintf(stderr, "\n"); failed = 1; \
    } \
} while (0)
    control_started = CreateEventW(NULL, TRUE, FALSE, NULL);
    HANDLE thread = CreateThread(NULL, 0, test_control_thread, NULL, 0, NULL);
    if (!thread || WaitForSingleObject(control_started, 5000) != WAIT_OBJECT_0 || !control_window) return 1;
    InterlockedExchange(&packet_count, 0);
    InterlockedExchange(&control_ready, 2);
    RECT rect;
    GetClientRect(window, &rect);
    int x = rect.right / 2, y = rect.bottom / 2;
    INPUT_CHECK(test_resize_borders(window), "All eight resize borders remain native with direct control enabled");
    INPUT_CHECK(test_resize_capture(window) && packet_count == 0,
        "Native resize must retain its mouse capture and must not send phone input");
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_MOUSEMOVE, MK_LBUTTON, MAKELPARAM(x + 20, y + 20));
    Sleep(80);
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x + 30, y + 30));
    INPUT_CHECK(packet_count >= 3 && packets[0].kind == POINTER_DOWN &&
        packets[1].kind == POINTER_MOVE && packets[2].kind == POINTER_UP &&
        packets[2].x > packets[0].x && packets[2].y > packets[0].y, "Drag sequence");

    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_MOUSEMOVE, MK_LBUTTON, MAKELPARAM(x + 10, y + 10));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x + 30, y + 30));
    INPUT_CHECK(packet_count == 3 && packets[0].kind == POINTER_DOWN &&
        packets[1].kind == POINTER_MOVE && packets[2].kind == POINTER_UP &&
        packets[1].x > packets[0].x && packets[2].x > packets[1].x, "Quick drag before movement timer");

    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDBLCLK, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(-50, -50));
    INPUT_CHECK((GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION) &&
        packet_count == 2 && packets[0].kind == POINTER_DOWN &&
        packets[1].kind == POINTER_UP && packets[1].x == 0 && packets[1].y == 0, "Double tap / outside release");

    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_CANCELMODE, 0, 0);
    INPUT_CHECK(packet_count == 2 && packets[1].kind == POINTER_CANCEL, "Cancel release");

    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    INPUT_CHECK(window_has_capture(window), "Phone drag must own mouse capture");
    SendMessageW(window, WM_ENTERSIZEMOVE, 0, 0);
    SendMessageW(window, WM_MOUSEMOVE, MK_LBUTTON, MAKELPARAM(x + 10, y + 10));
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_TIMER, 4107, 0);
    INPUT_CHECK(packet_count == 2 && packets[1].kind == POINTER_CANCEL && !window_has_capture(window),
        "Entering native move/resize cancels a phone drag and suppresses new input");
    SendMessageW(window, WM_EXITSIZEMOVE, 0, 0);
    InterlockedExchange(&packet_count, 0);
    InterlockedExchange(&reject_press, 1);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    INPUT_CHECK(!window_has_capture(window), "A failed phone press must release its capture");
    InterlockedExchange(&reject_press, 0);

    POINT screen = {x, y};
    ClientToScreen(window, &screen);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_MOUSEWHEEL, MAKEWPARAM(0, -WHEEL_DELTA), MAKELPARAM(screen.x, screen.y));
    INPUT_CHECK(packet_count == 1 && packets[0].kind == POINTER_WHEEL && packets[0].delta == -1, "Wheel");

    SendMessageW(window, WM_CANCELMODE, 0, 0);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_KEYDOWN, VK_CONTROL, 0x1d0001);
    SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
    SendMessageW(window, WM_KEYDOWN, 'A', 0x401e0001);
    SendMessageW(window, WM_KEYUP, 'A', (LPARAM)0xc01e0001);
    SendMessageW(window, WM_KEYUP, VK_CONTROL, (LPARAM)0xc01d0001);
    INPUT_CHECK(packet_count == 4 && packets[0].kind == KEY_DOWN && packets[0].x == 224 &&
        packets[1].kind == KEY_DOWN && packets[1].x == 4 && packets[2].kind == KEY_UP &&
        packets[2].x == 4 && packets[3].kind == KEY_UP && packets[3].x == 224,
        "Keyboard combo, physical key mapping, held-key repeat and release");
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_KEYDOWN, VK_SHIFT, 0x360001);
    SendMessageW(window, WM_KILLFOCUS, 0, 0);
    SendMessageW(window, WM_KEYUP, VK_SHIFT, (LPARAM)0xc0360001);
    INPUT_CHECK(packet_count == 2 && packets[0].kind == KEY_DOWN && packets[0].x == 229 &&
        packets[1].kind == POINTER_CANCEL, "Focus loss releases keys and ignores stale key-up");
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_KEYDOWN, VK_SHIFT, 0x40360001);
    INPUT_CHECK(packet_count == 0, "Held keys must not revive from desktop repeat after cancellation");
    INPUT_CHECK(test_focus(window, FALSE), "Test can remove keyboard focus on the native owner thread");
    SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
    INPUT_CHECK(packet_count == 0, "Unfocused video must not forward keyboard input");
    INPUT_CHECK(test_focus(window, TRUE), "Test can restore video focus");
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_SYSKEYDOWN, VK_MENU, 0x380001);
    SendMessageW(window, WM_SYSKEYDOWN, VK_RETURN, 0x201c0001);
    INPUT_CHECK(!(GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION) && packet_count == 2 &&
        packets[0].kind == KEY_DOWN && packets[1].kind == POINTER_CANCEL,
        "Alt+Enter stays local and releases the phone's Alt key");
    SendMessageW(window, WM_KEYDOWN, VK_ESCAPE, 0x010001);
    INPUT_CHECK(GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION, "Esc exits fullscreen locally");
    InterlockedExchange(&keyboard_ready, 0);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
    SendMessageW(window, WM_KEYUP, 'A', (LPARAM)0xc01e0001);
    INPUT_CHECK(packet_count == 0, "Missing keyboard subscription must not forward keys");
    InterlockedExchange(&keyboard_ready, 1);

    SendMessageW(window, WM_COMMAND, 4106, 0);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x, y));
    SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
    SendMessageW(window, WM_KEYUP, 'A', (LPARAM)0xc01e0001);
    INPUT_CHECK(packet_count == 0, "Disabled direct control");
    SendMessageW(window, WM_COMMAND, 4106, 0);

    InterlockedExchange(&control_ready, 1);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x, y));
    SendMessageW(window, WM_KEYDOWN, 'A', 0x1e0001);
    INPUT_CHECK(packet_count == 0, "Relative-only host");
    InterlockedExchange(&control_ready, 2);

    RECT square = {0, 0, 600, 600};
    AdjustWindowRectEx(&square, GetWindowLongPtrW(window, GWL_STYLE), FALSE, GetWindowLongPtrW(window, GWL_EXSTYLE));
    SetWindowPos(window, NULL, 0, 0, square.right-square.left, square.bottom-square.top,
        SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(300, 5));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(300, 5));
    INPUT_CHECK(packet_count == 0, "Letterbox rejection");
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(300, 300));
    SendMessageW(window, WM_SIZE, SIZE_RESTORED, MAKELPARAM(600, 600));
    INPUT_CHECK(packet_count == 2 && packets[1].kind == POINTER_CANCEL, "Resize release");

    PostMessageW(control_window, WM_CLOSE, 0, 0);
    failed |= WaitForSingleObject(thread, 5000) != WAIT_OBJECT_0;
    CloseHandle(thread);
    CloseHandle(control_started);
    if (failed) fprintf(stderr, "Direct video input failed: mapping, drag, double tap, release, wheel or connection guard\n");
#undef INPUT_CHECK
    return failed;
}

static int push_frame(GstElement *source, int width, int height, int sequence)
{
    GstCaps *caps = gst_caps_new_simple("video/x-raw", "format", G_TYPE_STRING, "BGRA",
        "width", G_TYPE_INT, width, "height", G_TYPE_INT, height,
        "framerate", GST_TYPE_FRACTION, 30, 1, NULL);
    gst_app_src_set_caps(GST_APP_SRC(source), caps);
    gst_caps_unref(caps);
    GstBuffer *frame = gst_buffer_new_allocate(NULL, width * height * 4, NULL);
    gst_buffer_memset(frame, 0, 0, width * height * 4);
    GST_BUFFER_PTS(frame) = sequence * GST_SECOND / 30;
    GST_BUFFER_DURATION(frame) = GST_SECOND / 30;
    return gst_app_src_push_buffer(GST_APP_SRC(source), frame) != GST_FLOW_OK;
}

static int wait_orientation(HWND window, int landscape)
{
    RECT rect;
    ULONGLONG deadline = GetTickCount64() + 5000;
    do {
        GetClientRect(window, &rect);
        if ((rect.right > rect.bottom) == landscape &&
            abs(rect.right * (landscape ? 270 : 480) -
                rect.bottom * (landscape ? 480 : 270)) < 1000) return 0;
        Sleep(20);
    } while (GetTickCount64() < deadline);
    fprintf(stderr, "Window did not follow %s video: %ldx%ld\n",
        landscape ? "landscape" : "portrait", rect.right, rect.bottom);
    return 1;
}

static int test_controls(HWND window, GstElement *source)
{
    if (wait_orientation(window, 0)) return 1;
    ariadne_video_client("Test iPhone");
    ariadne_video_metadata("Test \"episode\"", "Test artist");
    Sleep(100);
    wchar_t title[256] = {0};
    GetWindowTextW(window, title, 256);
    if (!wcsstr(title, L"Test \"episode\"") || !wcsstr(title, L"Test artist")) {
        fprintf(stderr, "Received metadata was not used for the title\n");
        return 1;
    }
    SendMessageW(window, WM_COMMAND, 4101, 0);
    if (!(GetWindowLongPtrW(window, GWL_EXSTYLE) & WS_EX_TOPMOST)) return 1;
    SendMessageW(window, WM_COMMAND, 4101, 0);
    if (GetWindowLongPtrW(window, GWL_EXSTYLE) & WS_EX_TOPMOST) return 1;

    RECT target = {0, 0, 300, 533};
    AdjustWindowRectEx(&target, GetWindowLongPtrW(window, GWL_STYLE), FALSE,
        GetWindowLongPtrW(window, GWL_EXSTYLE));
    SetWindowPos(window, NULL, 0, 0, target.right-target.left, target.bottom-target.top,
        SWP_NOMOVE | SWP_NOZORDER | SWP_NOACTIVATE);
    SendMessageW(window, WM_EXITSIZEMOVE, 0, 0);
    if (push_frame(source, 480, 270, 1) || wait_orientation(window, 1)) return 1;
    if (push_frame(source, 270, 480, 2) || wait_orientation(window, 0)) return 1;
    RECT restored;
    GetClientRect(window, &restored);
    if (abs(restored.right - 300) > 2 || abs(restored.bottom - 533) > 2) {
        fprintf(stderr, "Portrait window size was not remembered\n");
        return 1;
    }
    SendMessageW(window, WM_COMMAND, 4102, 0);
    if (push_frame(source, 480, 270, 3)) return 1;
    Sleep(100);
    RECT unchanged;
    GetClientRect(window, &unchanged);
    if (!EqualRect(&restored, &unchanged)) {
        fprintf(stderr, "Disabled auto-fit must preserve the user's window size\n");
        return 1;
    }
    SendMessageW(window, WM_COMMAND, 4102, 0);
    if (wait_orientation(window, 1)) return 1;
    ShowWindow(window, SW_MAXIMIZE);
    RECT maximized;
    GetWindowRect(window, &maximized);
    if (push_frame(source, 270, 480, 4)) return 1;
    Sleep(100);
    GetWindowRect(window, &unchanged);
    if (!IsZoomed(window) || !EqualRect(&maximized, &unchanged)) {
        fprintf(stderr, "Rotation must preserve maximized window\n");
        return 1;
    }
    ShowWindow(window, SW_RESTORE);
    SendMessageW(window, WM_COMMAND, 4104, 0);
    if (wait_orientation(window, 0)) return 1;
    SendMessageW(window, WM_LBUTTONDBLCLK, 0, 0);
    if (GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION) return 1;
    if (push_frame(source, 480, 270, 5)) return 1;
    Sleep(100);
    MONITORINFO monitor = {.cbSize = sizeof(monitor)};
    GetMonitorInfoW(MonitorFromWindow(window, MONITOR_DEFAULTTONEAREST), &monitor);
    RECT full;
    GetWindowRect(window, &full);
    if (!EqualRect(&full, &monitor.rcMonitor)) {
        fprintf(stderr, "Rotation must not resize fullscreen window\n");
        return 1;
    }
    SendMessageW(window, WM_KEYDOWN, VK_ESCAPE, 0);
    if (!(GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION) || wait_orientation(window, 1)) return 1;
    SendMessageW(window, WM_SYSKEYDOWN, VK_RETURN, 1 << 29);
    if (GetWindowLongPtrW(window, GWL_STYLE) & WS_CAPTION) return 1;
    SendMessageW(window, WM_KEYDOWN, VK_ESCAPE, 0);
    if (test_direct_input(window)) return 1;
    ariadne_video_stopped();
    Sleep(100);
    GetWindowTextW(window, title, 256);
    if (wcscmp(title, L"Ariadne")) {
        fprintf(stderr, "Disconnected window retained stale metadata\n");
        return 1;
    }
    return 0;
}

static GstBusSyncReply brand_window(GstBus *bus, GstMessage *message, gpointer unused)
{
    HWND window = ariadne_brand_video_window(message);
    if (window) {
        InterlockedExchangePointer(&video_window, window);
    }
    return GST_BUS_PASS;
}

static int test_window(void)
{
    GError *error = NULL;
    GstElement *pipeline = gst_parse_launch(
        "appsrc name=source is-live=true format=time "
        "! d3d11videosink",
        &error);
    if (!pipeline || error) {
        fprintf(stderr, "Could not create video test pipeline: %s\n", error ? error->message : "unknown");
        g_clear_error(&error);
        if (pipeline) gst_object_unref(pipeline);
        return 1;
    }
    InterlockedExchangePointer(&video_window, NULL);
    GstBus *bus = gst_element_get_bus(pipeline);
    gst_bus_set_sync_handler(bus, brand_window, NULL, NULL);
    GstElement *source = gst_bin_get_by_name(GST_BIN(pipeline), "source");
    gst_element_set_state(pipeline, GST_STATE_PLAYING);
    push_frame(source, 270, 480, 0);

    ULONGLONG deadline = GetTickCount64() + 10000;
    HWND window = NULL;
    do {
        window = InterlockedCompareExchangePointer(&video_window, NULL, NULL);
        if (window && IsWindowVisible(window)) break;
        Sleep(10);
    } while (GetTickCount64() < deadline);

    wchar_t title[128] = {0};
    if (window) GetWindowTextW(window, title, 128);
    HINSTANCE module = GetModuleHandleW(NULL);
    HICON large = LoadImageW(module, MAKEINTRESOURCEW(101), IMAGE_ICON,
                            GetSystemMetrics(SM_CXICON), GetSystemMetrics(SM_CYICON), LR_SHARED);
    HICON small = LoadImageW(module, MAKEINTRESOURCEW(101), IMAGE_ICON,
                            GetSystemMetrics(SM_CXSMICON), GetSystemMetrics(SM_CYSMICON), LR_SHARED);
    int failed = !window || !IsWindowVisible(window) || wcscmp(title, L"Ariadne") ||
                 !large || !small ||
                 (HICON) SendMessageW(window, WM_GETICON, ICON_BIG, 0) != large ||
                 (HICON) SendMessageW(window, WM_GETICON, ICON_SMALL, 0) != small;
    if (failed) {
        fprintf(stderr, "Video window must show Ariadne title and both embedded icons\n");
        GstMessage *message = gst_bus_pop_filtered(bus, GST_MESSAGE_ERROR);
        if (message) {
            gst_message_parse_error(message, &error, NULL);
            fprintf(stderr, "%s\n", error->message);
            g_clear_error(&error);
            gst_message_unref(message);
        }
    }
    if (!failed) failed = test_controls(window, source);
    gst_element_set_state(pipeline, GST_STATE_NULL);
    gst_object_unref(source);
    gst_object_unref(bus);
    gst_object_unref(pipeline);
    return failed;
}

int main(int argc, char **argv)
{
    gst_init(&argc, &argv);
    if (test_pointer_mapping()) { fprintf(stderr, "Pointer mapping failed\n"); return 1; }
    if (test_key_mapping()) return 1;
    GError *error = NULL;
    gchar *directory = g_dir_make_tmp("ariadne-video-test-XXXXXX", &error);
    if (!directory || !SetCurrentDirectoryA(directory)) return 1;
    for (int connection = 0; connection < 2; connection++) {
        if (test_window()) return 1;
    }
    DeleteFileW(L"window.ini");
    SetCurrentDirectoryW(L"..");
    RemoveDirectoryA(directory);
    g_free(directory);
    puts("PASS Ariadne icons, rotation, remembered sizes, metadata, topmost, fullscreen, resize capture, direct pointer input, keyboard and reconnect");
    gst_deinit();
    return 0;
}

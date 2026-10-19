#include <gst/app/gstappsrc.h>
#include <stdio.h>
#include "ariadne-window.h"

static PVOID volatile video_window;
static HANDLE control_started;
static HWND control_window;
static volatile LONG control_ready = 2, packet_count;
static AriadnePointerPacket packets[64];

static LRESULT CALLBACK test_control_proc(HWND window, UINT message, WPARAM wp, LPARAM lp)
{
    if (message == WM_COPYDATA) {
        COPYDATASTRUCT *data = (COPYDATASTRUCT *)lp;
        if (data->dwData != ARIADNE_POINTER_TAG || data->cbData != sizeof(AriadnePointerPacket)) return 0;
        AriadnePointerPacket *packet = data->lpData;
        if (packet->version != 1) return 0;
        if (packet->kind == POINTER_QUERY) return control_ready;
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

    POINT screen = {x, y};
    ClientToScreen(window, &screen);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_MOUSEWHEEL, MAKEWPARAM(0, -WHEEL_DELTA), MAKELPARAM(screen.x, screen.y));
    INPUT_CHECK(packet_count == 1 && packets[0].kind == POINTER_WHEEL && packets[0].delta == -1, "Wheel");

    SendMessageW(window, WM_COMMAND, 4106, 0);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x, y));
    INPUT_CHECK(packet_count == 0, "Disabled direct control");
    SendMessageW(window, WM_COMMAND, 4106, 0);

    InterlockedExchange(&control_ready, 1);
    InterlockedExchange(&packet_count, 0);
    SendMessageW(window, WM_LBUTTONDOWN, MK_LBUTTON, MAKELPARAM(x, y));
    SendMessageW(window, WM_LBUTTONUP, 0, MAKELPARAM(x, y));
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
    puts("PASS Ariadne icons, rotation, remembered sizes, metadata, topmost, fullscreen, direct pointer input and reconnect");
    gst_deinit();
    return 0;
}

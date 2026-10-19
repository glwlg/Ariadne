// SPDX-License-Identifier: GPL-3.0-or-later
#include "ariadne-window.h"
#include <gst/video/video.h>
#include <math.h>
#include <stdio.h>

#define WINDOW_PROPERTY L"Ariadne.VideoWindow"
#define VIDEO_SIZE (WM_APP + 201)
#define VIDEO_FRAME (WM_APP + 202)
#define VIDEO_TITLE (WM_APP + 203)
#define VIDEO_STOP (WM_APP + 204)
#define VIDEO_ASPECT (WM_APP + 205)
#define MENU_PIN 4101
#define MENU_FIT 4102
#define MENU_FULLSCREEN 4103
#define MENU_ADAPT 4104
#define READY_TIMER 4105
#define MENU_CONTROL 4106
#define POINTER_TIMER 4107

typedef struct {
    WNDPROC original;
    guint generation;
    int width, height, par_n, par_d;
    gboolean fullscreen, topmost, auto_fit, streaming, awake;
    WINDOWPLACEMENT placement;
    LONG_PTR style;
    RECT remembered[2];
    wchar_t preferences[MAX_PATH];
    GstElement *sink;
    GstPad *pad;
    gulong pad_probe, present_handler;
    gboolean direct_control, pointer_down, pointer_pending, input_active;
    POINT pointer;
    int wheel_delta;
} VideoWindow;

typedef struct {
    HWND window;
    guint generation;
    gboolean first_frame;
} StreamProbe;

G_LOCK_DEFINE_STATIC(video_state);
static HWND active_window;
static char *content_title, *content_artist, *client_name;
static gint next_generation;

gboolean ariadne_map_pointer(int cw, int ch, int vw, int vh, int par_n, int par_d,
                            int x, int y, gboolean clamp, POINT *point)
{
    if (cw <= 0 || ch <= 0 || vw <= 0 || vh <= 0 || par_n <= 0 || par_d <= 0) return FALSE;
    double ratio = (double)vw * par_n / ((double)vh * par_d);
    double w = fmin(cw, ch * ratio), h = w / ratio;
    double left = (cw - w) / 2, top = (ch - h) / 2;
    if (!clamp && (x < left || x >= left + w || y < top || y >= top + h)) return FALSE;
    point->x = (LONG)lround(fmax(0, fmin(1, (x - left) / fmax(1, w - 1))) * 32767);
    point->y = (LONG)lround(fmax(0, fmin(1, (y - top) / fmax(1, h - 1))) * 32767);
    return TRUE;
}

static int send_pointer(HWND window, int kind, POINT point, int delta)
{
    HWND control = FindWindowW(NULL, ARIADNE_CONTROL_TITLE);
    if (!control) return 0;
    AriadnePointerPacket packet = {1, kind, point.x, point.y, delta};
    COPYDATASTRUCT data = {ARIADNE_POINTER_TAG, sizeof(packet), &packet};
    DWORD_PTR result = 0;
    if (!SendMessageTimeoutW(control, WM_COPYDATA, (WPARAM)window, (LPARAM)&data,
        SMTO_ABORTIFHUNG | SMTO_BLOCK, 20, &result)) return 0;
    return (int)result;
}

static gboolean control_ready(HWND window, VideoWindow *state)
{
    return state->streaming && state->direct_control &&
        send_pointer(window, POINTER_QUERY, (POINT){0, 0}, 0) == 2;
}

static gboolean map_pointer(HWND window, VideoWindow *state, LPARAM position, gboolean clamp, POINT *point)
{
    RECT client;
    GetClientRect(window, &client);
    return ariadne_map_pointer(client.right, client.bottom, state->width, state->height,
        state->par_n, state->par_d, (short)LOWORD(position), (short)HIWORD(position), clamp, point);
}

static void cancel_pointer(HWND window, VideoWindow *state)
{
    KillTimer(window, POINTER_TIMER);
    state->pointer_pending = FALSE;
    state->pointer_down = FALSE;
    state->wheel_delta = 0;
    if (state->input_active) send_pointer(window, POINTER_CANCEL, (POINT){0, 0}, 0);
    state->input_active = FALSE;
    if (GetCapture() == window) ReleaseCapture();
}

static gboolean pointer_button(HWND window, VideoWindow *state, LPARAM position, gboolean down)
{
    POINT point;
    if (down && !control_ready(window, state)) return FALSE;
    if (!down && !state->pointer_down) return FALSE;
    if (!map_pointer(window, state, position, !down, &point)) return FALSE;
    // A quick drag may end before the movement timer fires; deliver its last move before release.
    if (!down && state->pointer_pending &&
        !send_pointer(window, POINTER_MOVE, state->pointer, 0)) {
        cancel_pointer(window, state);
        return TRUE;
    }
    state->pointer_pending = FALSE;
    // Activate before sending the press: moving focus away from the control panel releases its input.
    if (down) { SetFocus(window); SetCapture(window); }
    if (!send_pointer(window, down ? POINTER_DOWN : POINTER_UP, point, 0)) {
        cancel_pointer(window, state);
        return TRUE;
    }
    state->input_active = TRUE;
    state->pointer_down = down;
    if (!down && GetCapture() == window) ReleaseCapture();
    return TRUE;
}

static void json_string(GString *json, const char *value)
{
    gchar *valid = g_utf8_make_valid(value ? value : "", -1);
    g_string_append_c(json, '"');
    for (const unsigned char *p = (const unsigned char *)valid; *p; p++) {
        if (*p == '"' || *p == '\\') g_string_append_c(json, '\\');
        if (*p < 32) g_string_append_printf(json, "\\u%04x", *p);
        else g_string_append_c(json, *p);
    }
    g_string_append_c(json, '"');
    g_free(valid);
}

static void publish(VideoWindow *state, const char *phase)
{
    GString *json = g_string_new("ARIADNE_VIDEO {\"phase\":");
    json_string(json, phase);
    g_string_append_printf(json, ",\"width\":%d,\"height\":%d,\"title\":", state->width, state->height);
    G_LOCK(video_state);
    json_string(json, state->streaming ? content_title : "");
    g_string_append(json, ",\"client\":");
    json_string(json, state->streaming ? client_name : "");
    G_UNLOCK(video_state);
    g_string_append_printf(json, ",\"awake\":%s}\n", state->awake ? "true" : "false");
    g_print("%s", json->str);
    fflush(stdout);
    g_string_free(json, TRUE);
}

static void update_title(HWND window, VideoWindow *state)
{
    GString *title = g_string_new("Ariadne");
    G_LOCK(video_state);
    if (state->streaming && content_title && *content_title) {
        g_string_append_printf(title, " · %s", content_title);
        if (content_artist && *content_artist) g_string_append_printf(title, " · %s", content_artist);
    }
    G_UNLOCK(video_state);
    gunichar2 *wide = g_utf8_to_utf16(title->str, -1, NULL, NULL, NULL);
    if (wide) SetWindowTextW(window, (LPCWSTR)wide);
    g_free(wide);
    g_string_free(title, TRUE);
}

static void power_state(HWND window, VideoWindow *state)
{
    gboolean awake = state->streaming && IsWindowVisible(window) && !IsIconic(window);
    if (awake != state->awake) {
        if (!SetThreadExecutionState(ES_CONTINUOUS |
            (awake ? ES_DISPLAY_REQUIRED | ES_SYSTEM_REQUIRED : 0))) {
            g_warning("Could not update Ariadne display power request: %lu", GetLastError());
            return;
        }
        state->awake = awake;
    }
}

static void remember_size(HWND window, VideoWindow *state)
{
    if (!state->width || state->fullscreen || IsZoomed(window) || IsIconic(window)) return;
    RECT rect;
    GetClientRect(window, &rect);
    int orientation = state->width > state->height;
    state->remembered[orientation] = rect;
    wchar_t value[32];
    const wchar_t *section = orientation ? L"Landscape" : L"Portrait";
    swprintf(value, 32, L"%ld", rect.right);
    WritePrivateProfileStringW(section, L"Width", value, state->preferences);
    swprintf(value, 32, L"%ld", rect.bottom);
    WritePrivateProfileStringW(section, L"Height", value, state->preferences);
}

static void fit_window(HWND window, VideoWindow *state)
{
    if (!state->width || !state->height || state->fullscreen || IsZoomed(window) || IsIconic(window)) return;
    MONITORINFO monitor = { .cbSize = sizeof(monitor) };
    if (!GetMonitorInfoW(MonitorFromWindow(window, MONITOR_DEFAULTTONEAREST), &monitor)) return;
    RECT current;
    GetWindowRect(window, &current);
    RECT saved = state->remembered[state->width > state->height];
    double ratio = (double)state->width * state->par_n / (state->height * state->par_d);
    int limit_w = monitor.rcWork.right - monitor.rcWork.left - 40;
    int limit_h = monitor.rcWork.bottom - monitor.rcWork.top - 80;
    double height = saved.bottom > 0 && saved.right > 0 ?
        fmin(saved.bottom, saved.right / ratio) : limit_h * 0.8;
    height = fmin(height, fmin(limit_h, limit_w / ratio));
    RECT target = { 0, 0, (LONG)lround(height * ratio), (LONG)lround(height) };
    AdjustWindowRectEx(&target, (DWORD)GetWindowLongPtrW(window, GWL_STYLE), FALSE,
                      (DWORD)GetWindowLongPtrW(window, GWL_EXSTYLE));
    int w = target.right - target.left, h = target.bottom - target.top;
    int x = (current.left + current.right - w) / 2;
    int y = (current.top + current.bottom - h) / 2;
    x = MAX(monitor.rcWork.left, MIN(x, monitor.rcWork.right - w));
    y = MAX(monitor.rcWork.top, MIN(y, monitor.rcWork.bottom - h));
    SetWindowPos(window, NULL, x, y, w, h, SWP_NOZORDER | SWP_NOACTIVATE);
}

static void toggle_fullscreen(HWND window, VideoWindow *state)
{
    if (!state->fullscreen) {
        state->placement.length = sizeof(state->placement);
        GetWindowPlacement(window, &state->placement);
        state->style = GetWindowLongPtrW(window, GWL_STYLE);
        MONITORINFO monitor = { .cbSize = sizeof(monitor) };
        if (!GetMonitorInfoW(MonitorFromWindow(window, MONITOR_DEFAULTTONEAREST), &monitor)) return;
        state->fullscreen = TRUE;
        SetWindowLongPtrW(window, GWL_STYLE, state->style & ~WS_OVERLAPPEDWINDOW);
        SetWindowPos(window, NULL, monitor.rcMonitor.left, monitor.rcMonitor.top,
            monitor.rcMonitor.right - monitor.rcMonitor.left,
            monitor.rcMonitor.bottom - monitor.rcMonitor.top,
            SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
    } else {
        state->fullscreen = FALSE;
        SetWindowLongPtrW(window, GWL_STYLE, state->style);
        SetWindowPlacement(window, &state->placement);
        SetWindowPos(window, NULL, 0, 0, 0, 0,
            SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
        if (state->auto_fit) fit_window(window, state);
    }
}

static LRESULT CALLBACK window_proc(HWND window, UINT message, WPARAM wparam, LPARAM lparam)
{
    VideoWindow *state = GetPropW(window, WINDOW_PROPERTY);
    if (!state) return DefWindowProcW(window, message, wparam, lparam);
    switch (message) {
    case VIDEO_SIZE:
        if (wparam == state->generation) {
            int width = LOWORD(lparam), height = HIWORD(lparam);
            if (width != state->width || height != state->height) {
                cancel_pointer(window, state);
                if (state->streaming) remember_size(window, state);
                state->width = width;
                state->height = height;
                if (state->auto_fit) fit_window(window, state);
                if (state->streaming) publish(state, "mirroring");
            }
        }
        return 0;
    case VIDEO_ASPECT:
        if (wparam == state->generation) {
            if (state->par_n != LOWORD(lparam) || state->par_d != HIWORD(lparam))
                cancel_pointer(window, state);
            state->par_n = LOWORD(lparam);
            state->par_d = HIWORD(lparam);
            if (state->auto_fit) fit_window(window, state);
        }
        return 0;
    case VIDEO_FRAME:
        if (wparam != state->generation) return 0;
        state->streaming = TRUE;
        if (!IsWindowVisible(window)) SetTimer(window, READY_TIMER, 100, NULL);
        else {
            if (state->auto_fit) fit_window(window, state);
            power_state(window, state);
            update_title(window, state);
            publish(state, "mirroring");
        }
        return 0;
    case WM_TIMER:
        if (wparam == POINTER_TIMER) {
            if (state->pointer_pending) {
                state->pointer_pending = FALSE;
                if (!control_ready(window, state) ||
                    !send_pointer(window, POINTER_MOVE, state->pointer, 0)) cancel_pointer(window, state);
                else state->input_active = TRUE;
            } else KillTimer(window, POINTER_TIMER);
            return 0;
        }
        if (wparam == READY_TIMER && IsWindowVisible(window)) {
            KillTimer(window, READY_TIMER);
            SendMessageW(window, VIDEO_FRAME, state->generation, 0);
            return 0;
        }
        break;
    case VIDEO_TITLE:
        update_title(window, state);
        if (state->streaming) publish(state, "mirroring");
        return 0;
    case VIDEO_STOP:
        cancel_pointer(window, state);
        state->streaming = FALSE;
        KillTimer(window, READY_TIMER);
        power_state(window, state);
        update_title(window, state);
        publish(state, "disconnected");
        return 0;
    case WM_EXITSIZEMOVE:
        remember_size(window, state);
        break;
    case WM_SIZE:
        cancel_pointer(window, state);
        power_state(window, state);
        break;
    case WM_SHOWWINDOW:
        power_state(window, state);
        break;
    case WM_MOUSEMOVE:
        if ((state->pointer_pending || control_ready(window, state)) &&
            map_pointer(window, state, lparam, state->pointer_down, &state->pointer)) {
            if (!state->pointer_pending) SetTimer(window, POINTER_TIMER, 25, NULL);
            state->pointer_pending = TRUE;
            return 0;
        }
        break;
    case WM_LBUTTONDOWN:
        if (pointer_button(window, state, lparam, TRUE)) return 0;
        break;
    case WM_LBUTTONUP:
        if (pointer_button(window, state, lparam, FALSE)) return 0;
        break;
    case WM_LBUTTONDBLCLK:
        if (pointer_button(window, state, lparam, TRUE)) return 0;
        toggle_fullscreen(window, state);
        return 0;
    case WM_MOUSEWHEEL: {
        POINT point = {(short)LOWORD(lparam), (short)HIWORD(lparam)};
        ScreenToClient(window, &point);
        if (!control_ready(window, state) || state->pointer_down ||
            !map_pointer(window, state, MAKELPARAM(point.x, point.y), FALSE, &point)) break;
        state->wheel_delta += (short)HIWORD(wparam);
        if (abs(state->wheel_delta) >= WHEEL_DELTA) {
            int delta = state->wheel_delta < 0 ? -1 : 1;
            state->wheel_delta %= WHEEL_DELTA;
            state->pointer_pending = FALSE;
            if (send_pointer(window, POINTER_WHEEL, point, delta)) state->input_active = TRUE;
            else cancel_pointer(window, state);
        }
        return 0;
    }
    case WM_KILLFOCUS:
        if (wparam && IsChild(window, (HWND)wparam)) break;
        cancel_pointer(window, state);
        break;
    case WM_CANCELMODE:
        cancel_pointer(window, state);
        break;
    case WM_CAPTURECHANGED:
        if (state->pointer_down) cancel_pointer(window, state);
        break;
    case WM_SYSKEYDOWN:
        if (wparam == VK_RETURN && (lparam & (1 << 29))) {
            if (!(lparam & (1 << 30))) toggle_fullscreen(window, state);
            return 0;
        }
        break;
    case WM_KEYDOWN:
        if (wparam == VK_ESCAPE) cancel_pointer(window, state);
        if (wparam == VK_ESCAPE && state->fullscreen) {
            toggle_fullscreen(window, state);
            return 0;
        }
        break;
    case WM_CONTEXTMENU: {
        cancel_pointer(window, state);
        HMENU menu = CreatePopupMenu();
        int ready = state->streaming ? send_pointer(window, POINTER_QUERY, (POINT){0, 0}, 0) : 0;
        AppendMenuW(menu, MF_STRING | (ready == 2 ? 0 : MF_GRAYED) |
            (ready == 2 && state->direct_control ? MF_CHECKED : 0), MENU_CONTROL,
            ready == 2 ? L"直接操作手机" : ready == 1 ? L"直接操作手机（需重新配对）" : L"直接操作手机（未连接）");
        AppendMenuW(menu, MF_STRING | (state->topmost ? MF_CHECKED : 0), MENU_PIN, L"窗口置顶");
        AppendMenuW(menu, MF_STRING | (state->auto_fit ? MF_CHECKED : 0), MENU_FIT, L"跟随横竖屏");
        AppendMenuW(menu, MF_STRING, MENU_ADAPT, L"适配画面");
        AppendMenuW(menu, MF_STRING, MENU_FULLSCREEN, state->fullscreen ? L"退出全屏\tEsc" : L"全屏\tAlt+Enter");
        POINT point = { (short)LOWORD(lparam), (short)HIWORD(lparam) };
        if (point.x == -1 && point.y == -1) GetCursorPos(&point);
        UINT selected = TrackPopupMenu(menu, TPM_RETURNCMD | TPM_RIGHTBUTTON,
            point.x, point.y, 0, window, NULL);
        DestroyMenu(menu);
        if (selected) SendMessageW(window, WM_COMMAND, selected, 0);
        return 0;
    }
    case WM_COMMAND:
        switch (LOWORD(wparam)) {
        case MENU_CONTROL:
            cancel_pointer(window, state);
            state->direct_control = !state->direct_control;
            WritePrivateProfileStringW(L"Window", L"DirectControl", state->direct_control ? L"1" : L"0", state->preferences);
            return 0;
        case MENU_PIN:
            state->topmost = !state->topmost;
            SetWindowPos(window, state->topmost ? HWND_TOPMOST : HWND_NOTOPMOST,
                0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
            WritePrivateProfileStringW(L"Window", L"Topmost", state->topmost ? L"1" : L"0", state->preferences);
            return 0;
        case MENU_FIT:
            state->auto_fit = !state->auto_fit;
            WritePrivateProfileStringW(L"Window", L"AutoFit", state->auto_fit ? L"1" : L"0", state->preferences);
            if (state->auto_fit) fit_window(window, state);
            return 0;
        case MENU_ADAPT:
            fit_window(window, state);
            return 0;
        case MENU_FULLSCREEN:
            toggle_fullscreen(window, state);
            return 0;
        }
        break;
    case WM_NCDESTROY: {
        WNDPROC original = state->original;
        cancel_pointer(window, state);
        KillTimer(window, READY_TIMER);
        state->streaming = FALSE;
        power_state(window, state);
        publish(state, "disconnected");
        G_LOCK(video_state);
        if (active_window == window) {
            active_window = NULL;
            g_clear_pointer(&content_title, g_free);
            g_clear_pointer(&content_artist, g_free);
            g_clear_pointer(&client_name, g_free);
        }
        G_UNLOCK(video_state);
        RemovePropW(window, WINDOW_PROPERTY);
        SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)original);
        if (state->pad) {
            gst_pad_remove_probe(state->pad, state->pad_probe);
            gst_object_unref(state->pad);
        }
        if (state->sink) {
            g_signal_handler_disconnect(state->sink, state->present_handler);
            gst_object_unref(state->sink);
        }
        g_free(state);
        return CallWindowProcW(original, window, message, wparam, lparam);
    }
    }
    return CallWindowProcW(state->original, window, message, wparam, lparam);
}

static void post_size(GstPad *pad, StreamProbe *probe, GstCaps *caps)
{
    GstVideoInfo info;
    if (caps && gst_video_info_from_caps(&info, caps)) {
        int width = GST_VIDEO_INFO_WIDTH(&info), height = GST_VIDEO_INFO_HEIGHT(&info);
        if (width > 0 && height > 0 && width <= 32767 && height <= 32767)
        {
            int pn = GST_VIDEO_INFO_PAR_N(&info), pd = GST_VIDEO_INFO_PAR_D(&info);
            if (pn > 0 && pd > 0 && pn <= 32767 && pd <= 32767)
                PostMessageW(probe->window, VIDEO_ASPECT, probe->generation, MAKELPARAM(pn, pd));
            PostMessageW(probe->window, VIDEO_SIZE, probe->generation, MAKELPARAM(width, height));
        }
    }
}

static GstPadProbeReturn video_probe(GstPad *pad, GstPadProbeInfo *info, gpointer data)
{
    StreamProbe *probe = data;
    if (GST_PAD_PROBE_INFO_TYPE(info) & GST_PAD_PROBE_TYPE_EVENT_DOWNSTREAM) {
        GstEvent *event = GST_PAD_PROBE_INFO_EVENT(info);
        if (GST_EVENT_TYPE(event) == GST_EVENT_CAPS) {
            GstCaps *caps;
            gst_event_parse_caps(event, &caps);
            post_size(pad, probe, caps);
        } else if (GST_EVENT_TYPE(event) == GST_EVENT_TAG) {
            GstTagList *tags;
            gchar *title = NULL, *artist = NULL;
            gst_event_parse_tag(event, &tags);
            gst_tag_list_get_string(tags, GST_TAG_TITLE, &title);
            gst_tag_list_get_string(tags, GST_TAG_ARTIST, &artist);
            if (title) ariadne_video_metadata(title, artist);
            g_free(title);
            g_free(artist);
        }
    }
    return GST_PAD_PROBE_OK;
}

static void frame_presented(GstElement *sink, gpointer device, gpointer texture, gpointer data)
{
    StreamProbe *probe = data;
    if (probe->first_frame) return;
    probe->first_frame = TRUE;
    GstPad *pad = gst_element_get_static_pad(sink, "sink");
    GstCaps *caps = gst_pad_get_current_caps(pad);
    post_size(pad, probe, caps);
    if (caps) gst_caps_unref(caps);
    gst_object_unref(pad);
    PostMessageW(probe->window, VIDEO_FRAME, probe->generation, 0);
}

static void free_signal_data(gpointer data, GClosure *closure)
{
    g_free(data);
}

HWND ariadne_brand_video_window(GstMessage *message)
{
    if (GST_MESSAGE_TYPE(message) != GST_MESSAGE_ELEMENT) return NULL;
    const GstStructure *structure = gst_message_get_structure(message);
    guint64 handle = 0;
    if (!structure || !gst_structure_has_name(structure, "have-window-handle") ||
        !gst_structure_get_uint64(structure, "window-handle", &handle) || !handle) return NULL;
    HWND window = (HWND)(guintptr)handle;
    if (GetPropW(window, WINDOW_PROPERTY)) return window;
    VideoWindow *state = g_new0(VideoWindow, 1);
    state->generation = g_atomic_int_add(&next_generation, 1) + 1;
    state->par_n = state->par_d = 1;
    GetFullPathNameW(L"window.ini", MAX_PATH, state->preferences, NULL);
    state->topmost = GetPrivateProfileIntW(L"Window", L"Topmost", 0, state->preferences) != 0;
    state->auto_fit = GetPrivateProfileIntW(L"Window", L"AutoFit", 1, state->preferences) != 0;
    state->direct_control = GetPrivateProfileIntW(L"Window", L"DirectControl", 1, state->preferences) != 0;
    for (int i = 0; i < 2; i++) {
        const wchar_t *section = i ? L"Landscape" : L"Portrait";
        int w = GetPrivateProfileIntW(section, L"Width", 0, state->preferences);
        int h = GetPrivateProfileIntW(section, L"Height", 0, state->preferences);
        if (w >= 100 && h >= 100 && w <= 32767 && h <= 32767)
            state->remembered[i] = (RECT){0, 0, w, h};
    }
    state->original = (WNDPROC)GetWindowLongPtrW(window, GWLP_WNDPROC);
    if (!SetPropW(window, WINDOW_PROPERTY, state)) { g_free(state); return NULL; }
    SetLastError(0);
    if (!SetWindowLongPtrW(window, GWLP_WNDPROC, (LONG_PTR)window_proc) && GetLastError()) {
        g_warning("Could not attach Ariadne video window controls: %lu", GetLastError());
        RemovePropW(window, WINDOW_PROPERTY);
        g_free(state);
        return NULL;
    }
    G_LOCK(video_state);
    active_window = window;
    G_UNLOCK(video_state);
    HINSTANCE module = GetModuleHandleW(NULL);
    HICON large = LoadImageW(module, MAKEINTRESOURCEW(101), IMAGE_ICON,
        GetSystemMetrics(SM_CXICON), GetSystemMetrics(SM_CYICON), LR_SHARED);
    HICON small = LoadImageW(module, MAKEINTRESOURCEW(101), IMAGE_ICON,
        GetSystemMetrics(SM_CXSMICON), GetSystemMetrics(SM_CYSMICON), LR_SHARED);
    SendMessageW(window, WM_SETICON, ICON_BIG, (LPARAM)large);
    SendMessageW(window, WM_SETICON, ICON_SMALL, (LPARAM)small);
    SetWindowTextW(window, L"Ariadne");
    if (state->topmost) SetWindowPos(window, HWND_TOPMOST, 0, 0, 0, 0,
        SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    if (GST_IS_ELEMENT(GST_MESSAGE_SRC(message))) {
        GstPad *pad = gst_element_get_static_pad(GST_ELEMENT(GST_MESSAGE_SRC(message)), "sink");
        if (pad) {
            state->sink = gst_object_ref(GST_MESSAGE_SRC(message));
            state->pad = pad;
            StreamProbe *probe = g_new0(StreamProbe, 1);
            probe->window = window;
            probe->generation = state->generation;
            state->pad_probe = gst_pad_add_probe(pad, GST_PAD_PROBE_TYPE_EVENT_DOWNSTREAM,
                                                 video_probe, probe, g_free);
            StreamProbe *present = g_new0(StreamProbe, 1);
            present->window = window;
            present->generation = state->generation;
            g_object_set(state->sink, "emit-present", TRUE, NULL);
            state->present_handler = g_signal_connect_data(state->sink, "present",
                G_CALLBACK(frame_presented), present, free_signal_data, 0);
        }
    }
    return window;
}

void ariadne_video_metadata(const char *title, const char *artist)
{
    G_LOCK(video_state);
    g_free(content_title);
    g_free(content_artist);
    content_title = g_strndup(title ? title : "", 1024);
    content_artist = g_strndup(artist ? artist : "", 512);
    if (active_window) PostMessageW(active_window, VIDEO_TITLE, 0, 0);
    G_UNLOCK(video_state);
}

void ariadne_video_client(const char *name)
{
    G_LOCK(video_state);
    g_free(client_name);
    client_name = g_strndup(name ? name : "", 256);
    G_UNLOCK(video_state);
}

void ariadne_video_stopped(void)
{
    G_LOCK(video_state);
    g_clear_pointer(&content_title, g_free);
    g_clear_pointer(&content_artist, g_free);
    if (active_window) PostMessageW(active_window, VIDEO_STOP, 0, 0);
    G_UNLOCK(video_state);
}

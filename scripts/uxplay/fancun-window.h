#pragma once

#include <gst/gst.h>
#include <windows.h>

HWND fancun_brand_video_window(GstMessage *message);
void fancun_video_metadata(const char *title, const char *artist);
void fancun_video_client(const char *name);
void fancun_video_stopped(void);

/* Fixed-width WM_COPYDATA wire format shared with ScreenPointerPacket.cs. */
#define FANCUN_POINTER_TAG 0x41524950
#ifndef FANCUN_CONTROL_TITLE
#define FANCUN_CONTROL_TITLE L"Fancun · iPhone 控制"
#endif
enum { POINTER_QUERY, POINTER_MOVE, POINTER_DOWN, POINTER_UP, POINTER_WHEEL, POINTER_CANCEL,
       KEY_DOWN, KEY_UP, KEYBOARD_QUERY };
typedef struct { int version, kind, x, y, delta; } FancunPointerPacket;
gboolean fancun_map_pointer(int cw, int ch, int vw, int vh, int par_n, int par_d,
                            int x, int y, gboolean clamp, POINT *point);
int fancun_key_usage(WPARAM key, LPARAM flags);

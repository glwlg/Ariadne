#pragma once

#include <gst/gst.h>
#include <windows.h>

HWND ariadne_brand_video_window(GstMessage *message);
void ariadne_video_metadata(const char *title, const char *artist);
void ariadne_video_client(const char *name);
void ariadne_video_stopped(void);

/* Fixed-width WM_COPYDATA wire format shared with ScreenPointerPacket.cs. */
#define ARIADNE_POINTER_TAG 0x41524950
#ifndef ARIADNE_CONTROL_TITLE
#define ARIADNE_CONTROL_TITLE L"Ariadne · iPhone 控制"
#endif
enum { POINTER_QUERY, POINTER_MOVE, POINTER_DOWN, POINTER_UP, POINTER_WHEEL, POINTER_CANCEL };
typedef struct { int version, kind, x, y, delta; } AriadnePointerPacket;
gboolean ariadne_map_pointer(int cw, int ch, int vw, int vh, int par_n, int par_d,
                            int x, int y, gboolean clamp, POINT *point);

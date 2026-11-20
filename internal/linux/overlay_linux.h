#ifndef WIS_OVERLAY_LINUX_H
#define WIS_OVERLAY_LINUX_H

#include <glib.h>
#include <cairo.h>

// Match the Win32 overlay's physical pixel geometry.
enum { WIS_OVERLAY_WIDTH = 120, WIS_OVERLAY_HEIGHT = 46, WIS_OVERLAY_BOTTOM = 80 };

// All rendering and window operations run on Wails' GTK thread.
void wis_linux_overlay_paint(cairo_t *cr);
gboolean wis_x11_overlay_show(void);
void wis_x11_overlay_draw(void);
void wis_x11_overlay_hide(void);
void wis_x11_overlay_close(void);

#endif

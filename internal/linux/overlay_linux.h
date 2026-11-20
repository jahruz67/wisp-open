#ifndef WIS_OVERLAY_LINUX_H
#define WIS_OVERLAY_LINUX_H

#include <glib.h>
#include <cairo.h>

// Slightly larger pill, positioned just above the Win32 overlay's bottom offset.
enum { WIS_OVERLAY_WIDTH = 140, WIS_OVERLAY_HEIGHT = 54, WIS_OVERLAY_BOTTOM = 88 };

// All rendering and window operations run on Wails' GTK thread.
void wis_linux_overlay_paint(cairo_t *cr);
gboolean wis_x11_overlay_show(void);
void wis_x11_overlay_draw(void);
void wis_x11_overlay_hide(void);
void wis_x11_overlay_close(void);

#endif

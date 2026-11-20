//go:build linux

#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/Xresource.h>
#include <X11/extensions/shape.h>
#include <X11/extensions/Xrandr.h>
#include <cairo-xlib.h>
#include <math.h>
#include <stdio.h>
#include <unistd.h>
#include "overlay_linux.h"

static Display *display;
static Window window;
static Colormap colormap;
static cairo_surface_t *surface;
static double scale = 1;

static double display_scale(void) {
    const char *resources = XResourceManagerString(display);
    if (!resources) return 1;
    XrmDatabase database = XrmGetStringDatabase(resources);
    if (!database) return 1;
    XrmValue value;
    char *type;
    double dpi = 96;
    if (XrmGetResource(database, "Xft.dpi", "Xft.Dpi", &type, &value) && value.addr)
        dpi = g_ascii_strtod(value.addr, NULL);
    XrmDestroyDatabase(database);
    // Keep dimensions within X11's wire-format limits, even with invalid Xresources.
    return isfinite(dpi) && dpi > 0 ? CLAMP(dpi / 96, 1.0 / 46, 32767.0 / 126) : 1;
}

static void position_overlay(void) {
    Window root = DefaultRootWindow(display), child, pointer_root;
    int x = 0, y = 0, win_x, win_y;
    unsigned int mask;
    if (!XQueryPointer(display, root, &pointer_root, &child, &x, &y, &win_x, &win_y, &mask))
        x = y = 0;
    int left = 0, top = 0;
    int width = DisplayWidth(display, DefaultScreen(display));
    int height = DisplayHeight(display, DefaultScreen(display));
    int count = 0;
    int major = 1, minor = 5;
    XRRMonitorInfo *monitors = NULL;
    if (XRRQueryVersion(display, &major, &minor) && (major > 1 || (major == 1 && minor >= 5)))
        monitors = XRRGetMonitors(display, root, True, &count);
    for (int i = 0; monitors && i < count; i++) {
        XRRMonitorInfo *monitor = &monitors[i];
        if (x >= monitor->x && x < monitor->x + monitor->width &&
            y >= monitor->y && y < monitor->y + monitor->height) {
            left = monitor->x;
            top = monitor->y;
            width = monitor->width;
            height = monitor->height;
            break;
        }
    }
    if (monitors) XRRFreeMonitors(monitors);
    XMoveWindow(display, window, left + (width - (int)lround(120 * scale)) / 2,
                top + height - (int)lround((46 + 80) * scale));
}

// A bounding shape also keeps the pill rounded on noncomposited X11 desktops.
// An empty input shape makes both X11 and XWayland windows click-through.
static void shape_overlay(void) {
    int height = (int)lround(46 * scale);
    int width = (int)lround(120 * scale);
    double radius = height / 2.0;
    XRectangle *rows = g_new(XRectangle, height);
    for (int y = 0; y < height; y++) {
        double dy = y + 0.5 - radius;
        int inset = (int)floor(radius - sqrt(MAX(0, radius * radius - dy * dy)));
        rows[y] = (XRectangle){inset, y, width - inset * 2, 1};
    }
    XShapeCombineRectangles(display, window, ShapeBounding, 0, 0, rows, height, ShapeSet, YXBanded);
    XShapeCombineRectangles(display, window, ShapeInput, 0, 0, NULL, 0, ShapeSet, Unsorted);
    g_free(rows);
}

static void configure_window(void) {
    XStoreName(display, window, "WIS Recording Overlay");
    XWMHints hints = {0};
    hints.flags = InputHint;
    hints.input = False;
    XSetWMHints(display, window, &hints);
    Atom notification = XInternAtom(display, "_NET_WM_WINDOW_TYPE_NOTIFICATION", False);
    XChangeProperty(display, window, XInternAtom(display, "_NET_WM_WINDOW_TYPE", False),
                    XA_ATOM, 32, PropModeReplace, (unsigned char *)&notification, 1);
    Atom states[] = {XInternAtom(display, "_NET_WM_STATE_ABOVE", False),
                     XInternAtom(display, "_NET_WM_STATE_STICKY", False)};
    XChangeProperty(display, window, XInternAtom(display, "_NET_WM_STATE", False),
                    XA_ATOM, 32, PropModeReplace, (unsigned char *)states, G_N_ELEMENTS(states));
    unsigned long pid = (unsigned long)getpid();
    XChangeProperty(display, window, XInternAtom(display, "_NET_WM_PID", False),
                    XA_CARDINAL, 32, PropModeReplace, (unsigned char *)&pid, 1);
}

static gboolean create_overlay(void) {
    display = XOpenDisplay(NULL);
    if (!display) {
        g_warning("WIS overlay could not connect to X11/XWayland. Ensure XWayland is enabled in this session.");
        return FALSE;
    }
    int major, minor;
    if (!XShapeQueryVersion(display, &major, &minor) || major < 1 || (major == 1 && minor < 1)) {
        g_warning("WIS overlay requires the X11 Shape input extension for click-through rendering.");
        wis_x11_overlay_close();
        return FALSE;
    }
    int screen = DefaultScreen(display);
    Window root = RootWindow(display, screen);
    XVisualInfo info;
    Visual *visual = DefaultVisual(display, screen);
    int depth = DefaultDepth(display, screen);
    char selection[32];
    snprintf(selection, sizeof(selection), "_NET_WM_CM_S%d", screen);
    gboolean composited = XGetSelectionOwner(display, XInternAtom(display, selection, False)) != None;
    if (composited && XMatchVisualInfo(display, screen, 32, TrueColor, &info)) {
        visual = info.visual;
        depth = info.depth;
    }
    colormap = XCreateColormap(display, root, visual, AllocNone);
    XSetWindowAttributes attributes = {0};
    attributes.override_redirect = True;
    attributes.colormap = colormap;
    attributes.background_pixel = 0;
    attributes.border_pixel = 0;
    scale = display_scale();
    window = XCreateWindow(display, root, 0, 0, (int)lround(120 * scale), (int)lround(46 * scale),
                           0, depth, InputOutput, visual,
                           CWOverrideRedirect | CWColormap | CWBackPixel | CWBorderPixel, &attributes);
    configure_window();
    shape_overlay();
    surface = cairo_xlib_surface_create(display, window, visual,
                                        (int)lround(120 * scale), (int)lround(46 * scale));
    if (cairo_surface_status(surface) != CAIRO_STATUS_SUCCESS) {
        g_warning("WIS overlay could not create its X11 drawing surface: %s",
                  cairo_status_to_string(cairo_surface_status(surface)));
        wis_x11_overlay_close();
        return FALSE;
    }
    return TRUE;
}

gboolean wis_x11_overlay_show(void) {
    if (!window && !create_overlay()) return FALSE;
    position_overlay();
    XMapRaised(display, window);
    wis_x11_overlay_draw();
    return TRUE;
}

void wis_x11_overlay_draw(void) {
    if (!window) return;
    // Drain events on our separate connection; GTK keeps its own display backend.
    while (XPending(display)) {
        XEvent event;
        XNextEvent(display, &event);
    }
    cairo_t *cr = cairo_create(surface);
    cairo_scale(cr, scale, scale);
    cairo_push_group(cr);
    wis_linux_overlay_paint(cr);
    cairo_pop_group_to_source(cr);
    cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
    cairo_paint(cr);
    cairo_destroy(cr);
    cairo_surface_flush(surface);
    XRaiseWindow(display, window);
    XFlush(display);
}

void wis_x11_overlay_hide(void) {
    if (!window) return;
    XUnmapWindow(display, window);
    XFlush(display);
}

void wis_x11_overlay_close(void) {
    if (surface) cairo_surface_destroy(surface);
    if (window) XDestroyWindow(display, window);
    if (colormap) XFreeColormap(display, colormap);
    if (display) XCloseDisplay(display);
    surface = NULL;
    window = 0;
    colormap = 0;
    display = NULL;
}

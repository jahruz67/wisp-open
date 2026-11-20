//go:build linux

#include <gtk/gtk.h>
#include <gtk-layer-shell.h>
#include <math.h>
#include <string.h>
#include "overlay_linux.h"

static GtkWidget *overlay;
static gchar *overlay_message;
static guint overlay_timer;
static GMutex volume_mutex;
static double overlay_volume;
static gboolean overlay_x11;

static void rounded_rect(cairo_t *cr, double x, double y, double w, double h, double radius) {
    cairo_new_sub_path(cr);
    cairo_arc(cr, x + w - radius, y + radius, radius, -G_PI / 2, 0);
    cairo_arc(cr, x + w - radius, y + h - radius, radius, 0, G_PI / 2);
    cairo_arc(cr, x + radius, y + h - radius, radius, G_PI / 2, G_PI);
    cairo_arc(cr, x + radius, y + radius, radius, G_PI, 3 * G_PI / 2);
    cairo_close_path(cr);
}

void wis_linux_overlay_paint(cairo_t *cr) {
    cairo_save(cr);
    cairo_scale(cr, WIS_OVERLAY_WIDTH / 120.0, WIS_OVERLAY_HEIGHT / 46.0);
    cairo_set_operator(cr, CAIRO_OPERATOR_SOURCE);
    cairo_set_source_rgba(cr, 0, 0, 0, 0);
    cairo_paint(cr);
    cairo_set_operator(cr, CAIRO_OPERATOR_OVER);
    cairo_set_source_rgba(cr, 18.0 / 255, 32.0 / 255, 51.0 / 255, 240.0 / 255);
    rounded_rect(cr, 0, 0, 120, 46, 23);
    cairo_fill(cr);

    gboolean recording = g_str_has_prefix(overlay_message, "Recording");
    gboolean processing = g_str_has_prefix(overlay_message, "Processing") ||
                          g_str_has_prefix(overlay_message, "Transcribing");
    if (!recording && !processing) {
        PangoLayout *text = pango_cairo_create_layout(cr);
        pango_layout_set_text(text, overlay_message, -1);
        PangoFontDescription *font = pango_font_description_from_string("Sans Semi-Bold 18");
        pango_font_description_set_absolute_size(font, 18 * PANGO_SCALE);
        pango_layout_set_font_description(text, font);
        pango_font_description_free(font);
        pango_layout_set_width(text, 112 * PANGO_SCALE);
        pango_layout_set_ellipsize(text, PANGO_ELLIPSIZE_END);
        pango_layout_set_alignment(text, PANGO_ALIGN_CENTER);
        int height;
        pango_layout_get_pixel_size(text, NULL, &height);
        cairo_set_source_rgb(cr, 220.0 / 255, 234.0 / 255, 1);
        if (g_str_equal(overlay_message, "Error") || strstr(overlay_message, "Key") || strstr(overlay_message, "Err"))
            cairo_set_source_rgb(cr, 1, 69.0 / 255, 69.0 / 255);
        cairo_move_to(cr, 4, (46 - height) / 2.0);
        pango_cairo_show_layout(cr, text);
        g_object_unref(text);
        cairo_restore(cr);
        return;
    }

    g_mutex_lock(&volume_mutex);
    double volume = overlay_volume;
    g_mutex_unlock(&volume_mutex);
    int count = recording ? 7 : 5;
    double gap = recording ? 4 : 6;
    double start = (120 - (count * 4 + (count - 1) * gap)) / 2.0;
    double phase = g_get_monotonic_time() / 1000000.0 * (recording ? 6 : 3);
    if (recording) cairo_set_source_rgb(cr, 220.0 / 255, 234.0 / 255, 1);
    else cairo_set_source_rgb(cr, 1, 0.8, 0);
    for (int i = 0; i < count; i++) {
        double height = recording ? MIN(38, 6 + 4 * fabs(sin(phase + i * 0.8)) + volume * 120)
                                  : 10 + 10 * fabs(sin(phase + i * 0.3));
        rounded_rect(cr, start + i * (4 + gap), (46 - height) / 2, 4, height, 2);
        cairo_fill(cr);
    }
    cairo_restore(cr);
}

static gboolean draw_overlay(GtkWidget *widget, cairo_t *cr, gpointer unused) {
    (void)unused;
    int scale = gtk_widget_get_scale_factor(widget);
    cairo_scale(cr, 1.0 / MAX(1, scale), 1.0 / MAX(1, scale));
    wis_linux_overlay_paint(cr);
    return TRUE;
}

static gboolean animate_overlay(gpointer unused) {
    (void)unused;
    if (overlay_x11) wis_x11_overlay_draw();
    else gtk_widget_queue_draw(overlay);
    return G_SOURCE_CONTINUE;
}

static GdkMonitor *overlay_monitor(void) {
    GdkDisplay *display = gdk_display_get_default();
    GdkSeat *seat = gdk_display_get_default_seat(display);
    GdkDevice *pointer = seat ? gdk_seat_get_pointer(seat) : NULL;
    if (pointer) {
        gint x, y;
        gdk_device_get_position(pointer, NULL, &x, &y);
        GdkMonitor *monitor = gdk_display_get_monitor_at_point(display, x, y);
        if (monitor) return monitor;
    }
    GdkMonitor *monitor = gdk_display_get_primary_monitor(display);
    return monitor ? monitor : gdk_display_get_monitor(display, 0);
}

static void size_overlay(GdkMonitor *monitor) {
    int scale = monitor ? MAX(1, gdk_monitor_get_scale_factor(monitor)) : 1;
    int width = MAX(1, (WIS_OVERLAY_WIDTH + scale / 2) / scale);
    int height = MAX(1, (WIS_OVERLAY_HEIGHT + scale / 2) / scale);
    // Layer-shell uses the widget's requested size, not GtkWindow's default size.
    gtk_widget_set_size_request(overlay, width, height);
    gtk_window_resize(GTK_WINDOW(overlay), width, height);
    gtk_layer_set_margin(GTK_WINDOW(overlay), GTK_LAYER_SHELL_EDGE_BOTTOM,
                         (WIS_OVERLAY_BOTTOM + scale / 2) / scale);
}

static void create_overlay(GdkMonitor *monitor) {
    overlay = gtk_window_new(GTK_WINDOW_TOPLEVEL);
    GtkWindow *window = GTK_WINDOW(overlay);
    gtk_window_set_title(window, "WIS Recording Overlay");
    gtk_window_set_decorated(window, FALSE);
    gtk_window_set_resizable(window, FALSE);
    gtk_window_set_accept_focus(window, FALSE);
    gtk_window_set_focus_on_map(window, FALSE);
    gtk_window_set_skip_taskbar_hint(window, TRUE);
    gtk_window_set_skip_pager_hint(window, TRUE);
    gtk_window_set_keep_above(window, TRUE);
    gtk_window_set_type_hint(window, GDK_WINDOW_TYPE_HINT_NOTIFICATION);
    gtk_widget_set_app_paintable(overlay, TRUE);
    GdkVisual *visual = gdk_screen_get_rgba_visual(gtk_widget_get_screen(overlay));
    if (visual) gtk_widget_set_visual(overlay, visual);

    gtk_layer_init_for_window(window);
    gtk_layer_set_namespace(window, "wis-free-v3-overlay");
    gtk_layer_set_layer(window, GTK_LAYER_SHELL_LAYER_OVERLAY);
    gtk_layer_set_monitor(window, monitor);
    gtk_layer_set_anchor(window, GTK_LAYER_SHELL_EDGE_BOTTOM, TRUE);
    // Position against the screen edge, independent of panel/work-area reservations.
    gtk_layer_set_exclusive_zone(window, -1);
    gtk_layer_set_keyboard_mode(window, GTK_LAYER_SHELL_KEYBOARD_MODE_NONE);
    size_overlay(monitor);
    g_signal_connect(overlay, "draw", G_CALLBACK(draw_overlay), NULL);
    gtk_widget_realize(overlay);
    cairo_region_t *empty = cairo_region_create();
    gdk_window_input_shape_combine_region(gtk_widget_get_window(overlay), empty, 0, 0);
    cairo_region_destroy(empty);
}

enum { OVERLAY_SHOW, OVERLAY_HIDE, OVERLAY_CLOSE };
typedef struct {
    int action;
    gchar *message;
} OverlayCommand;

static void stop_overlay(void) {
    if (overlay_timer) {
        g_source_remove(overlay_timer);
        overlay_timer = 0;
    }
    if (overlay) gtk_widget_hide(overlay);
    if (overlay_x11) wis_x11_overlay_hide();
}

static gboolean show_overlay(void) {
    if (!gtk_layer_is_supported()) {
        overlay_x11 = TRUE;
        return wis_x11_overlay_show();
    }
    GdkMonitor *monitor = overlay_monitor();
    if (!overlay) create_overlay(monitor);
    if (monitor && gtk_layer_get_monitor(GTK_WINDOW(overlay)) != monitor)
        gtk_layer_set_monitor(GTK_WINDOW(overlay), monitor);
    size_overlay(monitor);
    gtk_widget_show(overlay);
    gtk_widget_queue_draw(overlay);
    return TRUE;
}

static gboolean apply_overlay_command(gpointer data) {
    OverlayCommand *command = data;
    if (command->action == OVERLAY_SHOW) {
        g_free(overlay_message);
        overlay_message = g_strdup(command->message);
        if (show_overlay() && !overlay_timer)
            overlay_timer = g_timeout_add(16, animate_overlay, NULL);
    } else {
        stop_overlay();
        if (command->action == OVERLAY_CLOSE) {
            if (overlay) gtk_widget_destroy(overlay);
            overlay = NULL;
            wis_x11_overlay_close();
            g_clear_pointer(&overlay_message, g_free);
        }
    }
    return G_SOURCE_REMOVE;
}

static void free_overlay_command(gpointer data) {
    OverlayCommand *command = data;
    g_free(command->message);
    g_free(command);
}

static void queue_overlay_command(int action, const char *message) {
    OverlayCommand *command = g_new0(OverlayCommand, 1);
    command->action = action;
    command->message = g_strdup(message);
    g_idle_add_full(G_PRIORITY_DEFAULT, apply_overlay_command, command, free_overlay_command);
}

void wis_linux_overlay_show(const char *message) { queue_overlay_command(OVERLAY_SHOW, message); }
void wis_linux_overlay_hide(void) { queue_overlay_command(OVERLAY_HIDE, NULL); }
void wis_linux_overlay_close(void) { queue_overlay_command(OVERLAY_CLOSE, NULL); }

void wis_linux_overlay_set_volume(double level) {
    g_mutex_lock(&volume_mutex);
    overlay_volume = overlay_volume * 0.9 + level * 0.1;
    g_mutex_unlock(&volume_mutex);
}

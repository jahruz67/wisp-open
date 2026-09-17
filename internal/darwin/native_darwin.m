//go:build darwin

#import <AppKit/AppKit.h>
#import <ApplicationServices/ApplicationServices.h>
#import <AVFoundation/AVFoundation.h>
#import <CoreAudio/CoreAudio.h>
#import <CoreGraphics/CoreGraphics.h>
#import <IOKit/hidsystem/ev_keymap.h>
#import <ServiceManagement/ServiceManagement.h>
#import <dispatch/dispatch.h>
#include <stdbool.h>
#include <math.h>
#include <stdlib.h>

static void WISRunOnMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) {
        block();
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

int wis_microphone_permission(void) {
    return (int)[AVCaptureDevice authorizationStatusForMediaType:AVMediaTypeAudio];
}

bool wis_accessibility_permission(void) {
    return AXIsProcessTrusted();
}

void wis_request_microphone_permission(void) {
    [AVCaptureDevice requestAccessForMediaType:AVMediaTypeAudio completionHandler:^(BOOL granted) {
        (void)granted;
    }];
}

void wis_request_accessibility_permission(void) {
    NSDictionary *options = @{(__bridge NSString *)kAXTrustedCheckOptionPrompt: @YES};
    AXIsProcessTrustedWithOptions((__bridge CFDictionaryRef)options);
}

void wis_open_permission_settings(const char *kind) {
    NSString *value = [NSString stringWithUTF8String:kind ?: ""];
    NSString *pane = [value isEqualToString:@"microphone"] ? @"Privacy_Microphone" : @"Privacy_Accessibility";
    NSString *raw = [NSString stringWithFormat:@"x-apple.systempreferences:com.apple.preference.security?%@", pane];
    WISRunOnMain(^{ [[NSWorkspace sharedWorkspace] openURL:[NSURL URLWithString:raw]]; });
}

void wis_set_settings_visible(bool visible) {
    WISRunOnMain(^{
        [NSApp setActivationPolicy:visible ? NSApplicationActivationPolicyRegular : NSApplicationActivationPolicyAccessory];
        if (visible) {
            [NSApp activateIgnoringOtherApps:YES];
        }
    });
}

char *wis_active_application_name(void) {
    NSRunningApplication *app = [[NSWorkspace sharedWorkspace] frontmostApplication];
    NSString *name = app.localizedName ?: @"";
    return strdup(name.UTF8String);
}

bool wis_paste(void) {
    if (!AXIsProcessTrusted()) return false;
    CGEventSourceRef source = CGEventSourceCreate(kCGEventSourceStateHIDSystemState);
    if (!source) return false;
    CGEventRef down = CGEventCreateKeyboardEvent(source, (CGKeyCode)9, true);
    CGEventRef up = CGEventCreateKeyboardEvent(source, (CGKeyCode)9, false);
    if (!down || !up) {
        if (down) CFRelease(down);
        if (up) CFRelease(up);
        CFRelease(source);
        return false;
    }
    CGEventSetFlags(down, kCGEventFlagMaskCommand);
    CGEventSetFlags(up, kCGEventFlagMaskCommand);
    CGEventPost(kCGSessionEventTap, down);
    CGEventPost(kCGSessionEventTap, up);
    CFRelease(down);
    CFRelease(up);
    CFRelease(source);
    return true;
}

bool wis_output_active(void) {
    AudioDeviceID device = kAudioObjectUnknown;
    UInt32 size = sizeof(device);
    AudioObjectPropertyAddress defaultOutput = {
        kAudioHardwarePropertyDefaultOutputDevice,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    if (AudioObjectGetPropertyData(kAudioObjectSystemObject, &defaultOutput, 0, NULL, &size, &device) != noErr || device == kAudioObjectUnknown) return false;
    UInt32 running = 0;
    size = sizeof(running);
    AudioObjectPropertyAddress isRunning = {
        kAudioDevicePropertyDeviceIsRunningSomewhere,
        kAudioObjectPropertyScopeGlobal,
        kAudioObjectPropertyElementMain
    };
    return AudioObjectGetPropertyData(device, &isRunning, 0, NULL, &size, &running) == noErr && running != 0;
}

void wis_toggle_media(void) {
    WISRunOnMain(^{
        for (int keyDown = 1; keyDown >= 0; keyDown--) {
            int flags = keyDown ? 0xA00 : 0xB00;
            NSEvent *event = [NSEvent otherEventWithType:NSEventTypeSystemDefined
                                               location:NSZeroPoint
                                          modifierFlags:flags
                                              timestamp:0
                                           windowNumber:0
                                                context:nil
                                                subtype:8
                                                  data1:(NX_KEYTYPE_PLAY << 16) | flags
                                                  data2:-1];
            CGEventRef cg = event.CGEvent;
            if (cg) CGEventPost(kCGSessionEventTap, cg);
        }
    });
}

static char *WISError(NSError *error) {
    return error ? strdup(error.localizedDescription.UTF8String) : NULL;
}

char *wis_add_to_startup(void) {
    NSError *error = nil;
    if (![SMAppService.mainAppService registerAndReturnError:&error]) return WISError(error);
    return NULL;
}

char *wis_remove_from_startup(void) {
    NSError *error = nil;
    if (![SMAppService.mainAppService unregisterAndReturnError:&error]) return WISError(error);
    return NULL;
}

bool wis_is_in_startup(void) {
    return SMAppService.mainAppService.status == SMAppServiceStatusEnabled;
}

@interface WISOverlayView : NSView
@property(nonatomic, copy) NSString *mode;
@property(nonatomic) double volume;
@property(nonatomic) double phase;
@end

@implementation WISOverlayView
- (BOOL)isOpaque { return NO; }
- (void)drawRect:(NSRect)dirtyRect {
    (void)dirtyRect;
    NSRect bounds = self.bounds;
    [[NSColor colorWithCalibratedRed:0.043 green:0.059 blue:0.102 alpha:0.94] setFill];
    [[NSBezierPath bezierPathWithRoundedRect:bounds xRadius:23 yRadius:23] fill];

    BOOL processing = [self.mode hasPrefix:@"Transcribing"] || [self.mode hasPrefix:@"Processing"] || [self.mode hasPrefix:@"Typing"];
    NSInteger count = processing ? 5 : 7;
    CGFloat width = 4.0;
    CGFloat gap = processing ? 6.0 : 4.0;
    CGFloat total = count * width + (count - 1) * gap;
    CGFloat start = (NSWidth(bounds) - total) / 2.0;
    NSColor *barColor = processing ? [NSColor colorWithSRGBRed:1 green:0.65 blue:0 alpha:1] : NSColor.whiteColor;
    [barColor setFill];
    for (NSInteger i = 0; i < count; i++) {
        double wave = fabs(sin(self.phase + i * (processing ? 0.45 : 0.8)));
        double height = processing ? 10.0 + wave * 12.0 : 7.0 + wave * 5.0 + MIN(self.volume * 110.0, 24.0);
        NSRect bar = NSMakeRect(start + i * (width + gap), (NSHeight(bounds) - height) / 2.0, width, height);
        [[NSBezierPath bezierPathWithRoundedRect:bar xRadius:2 yRadius:2] fill];
    }
}
@end

static NSPanel *wisOverlayPanel;
static WISOverlayView *wisOverlayView;
static NSTimer *wisOverlayTimer;

static NSScreen *WISScreenAtPoint(NSPoint point) {
    for (NSScreen *screen in NSScreen.screens) {
        if (NSPointInRect(point, screen.frame)) return screen;
    }
    return NSScreen.mainScreen;
}

void wis_overlay_create(void) {
    WISRunOnMain(^{
        if (wisOverlayPanel) return;
        NSRect frame = NSMakeRect(0, 0, 120, 46);
        wisOverlayPanel = [[NSPanel alloc] initWithContentRect:frame
                                                    styleMask:NSWindowStyleMaskBorderless | NSWindowStyleMaskNonactivatingPanel
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO];
        wisOverlayPanel.opaque = NO;
        wisOverlayPanel.backgroundColor = NSColor.clearColor;
        wisOverlayPanel.hasShadow = YES;
        wisOverlayPanel.ignoresMouseEvents = YES;
        wisOverlayPanel.level = NSStatusWindowLevel;
        wisOverlayPanel.collectionBehavior = NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary;
        wisOverlayView = [[WISOverlayView alloc] initWithFrame:frame];
        wisOverlayView.mode = @"Recording";
        wisOverlayPanel.contentView = wisOverlayView;
        wisOverlayTimer = [NSTimer scheduledTimerWithTimeInterval:(1.0 / 30.0) repeats:YES block:^(NSTimer *timer) {
            (void)timer;
            if (!wisOverlayPanel.isVisible) return;
            wisOverlayView.phase += 0.18;
            [wisOverlayView setNeedsDisplay:YES];
        }];
    });
}

void wis_overlay_show(const char *message) {
    NSString *mode = [NSString stringWithUTF8String:message ?: "Recording"];
    WISRunOnMain(^{
        if (!wisOverlayPanel) wis_overlay_create();
        wisOverlayView.mode = mode;
        NSScreen *screen = WISScreenAtPoint(NSEvent.mouseLocation);
        NSRect visible = screen.visibleFrame;
        [wisOverlayPanel setFrameOrigin:NSMakePoint(NSMidX(visible) - 60, NSMinY(visible) + 60)];
        [wisOverlayPanel orderFrontRegardless];
    });
}

void wis_overlay_hide(void) { WISRunOnMain(^{ [wisOverlayPanel orderOut:nil]; }); }

void wis_overlay_set_volume(double level) {
    WISRunOnMain(^{ wisOverlayView.volume = wisOverlayView.volume * 0.9 + level * 0.1; });
}

void wis_overlay_close(void) {
    WISRunOnMain(^{
        [wisOverlayTimer invalidate];
        wisOverlayTimer = nil;
        [wisOverlayPanel close];
        wisOverlayPanel = nil;
        wisOverlayView = nil;
    });
}

//go:build darwin

#import <AppKit/AppKit.h>
#import <dispatch/dispatch.h>
#include <stdbool.h>

extern void goWISTraySettings(void);
extern void goWISTrayToggleStartup(void);
extern void goWISTrayQuit(void);

static NSStatusItem *wisStatusItem;
static NSMenuItem *wisStatusMenuItem;
static NSMenuItem *wisCountMenuItem;
static NSMenuItem *wisStartupMenuItem;
static NSString *wisTrayLabel;

@interface WISTrayTarget : NSObject
- (void)openSettings:(id)sender;
- (void)toggleStartup:(id)sender;
- (void)quit:(id)sender;
@end

@implementation WISTrayTarget
- (void)openSettings:(id)sender { (void)sender; goWISTraySettings(); }
- (void)toggleStartup:(id)sender { (void)sender; goWISTrayToggleStartup(); }
- (void)quit:(id)sender { (void)sender; goWISTrayQuit(); }
@end

static WISTrayTarget *wisTrayTarget;

static void WISTrayOnMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) block();
    else dispatch_async(dispatch_get_main_queue(), block);
}

void wis_tray_start(const char *label, const char *shortcut, bool startup, const void *icon, int iconLen) {
    NSString *labelValue = [NSString stringWithUTF8String:label ?: "wis-free-v3"];
    NSString *shortcutValue = [NSString stringWithUTF8String:shortcut ?: "alt+z"];
    NSData *iconCopy = icon && iconLen > 0 ? [NSData dataWithBytes:icon length:(NSUInteger)iconLen] : nil;
    WISTrayOnMain(^{
        if (wisStatusItem) return;
        wisTrayLabel = labelValue;
        wisTrayTarget = [WISTrayTarget new];
        wisStatusItem = [NSStatusBar.systemStatusBar statusItemWithLength:NSSquareStatusItemLength];
        NSImage *image = [[NSImage alloc] initWithData:iconCopy];
        image.size = NSMakeSize(18, 18);
        image.template = YES;
        wisStatusItem.button.image = image;
        wisStatusItem.button.toolTip = [NSString stringWithFormat:@"%@ — %@ to record", labelValue, shortcutValue];

        NSMenu *menu = [NSMenu new];
        wisStatusMenuItem = [[NSMenuItem alloc] initWithTitle:@"Status: Ready" action:nil keyEquivalent:@""];
        wisStatusMenuItem.enabled = NO;
        [menu addItem:wisStatusMenuItem];
        wisCountMenuItem = [[NSMenuItem alloc] initWithTitle:@"Shortcut detected: 0 times" action:nil keyEquivalent:@""];
        wisCountMenuItem.enabled = NO;
        [menu addItem:wisCountMenuItem];
        [menu addItem:NSMenuItem.separatorItem];

        NSMenuItem *settings = [[NSMenuItem alloc] initWithTitle:@"Settings" action:@selector(openSettings:) keyEquivalent:@""];
        settings.target = wisTrayTarget;
        [menu addItem:settings];
        wisStartupMenuItem = [[NSMenuItem alloc] initWithTitle:@"Start at Login" action:@selector(toggleStartup:) keyEquivalent:@""];
        wisStartupMenuItem.target = wisTrayTarget;
        wisStartupMenuItem.state = startup ? NSControlStateValueOn : NSControlStateValueOff;
        [menu addItem:wisStartupMenuItem];
        [menu addItem:NSMenuItem.separatorItem];
        NSMenuItem *quit = [[NSMenuItem alloc] initWithTitle:@"Exit" action:@selector(quit:) keyEquivalent:@"q"];
        quit.target = wisTrayTarget;
        [menu addItem:quit];
        wisStatusItem.menu = menu;
    });
}

void wis_tray_set_status(const char *status, const void *icon, int iconLen, bool templateIcon) {
    NSString *statusValue = [NSString stringWithUTF8String:status ?: "Ready"];
    NSData *iconCopy = icon && iconLen > 0 ? [NSData dataWithBytes:icon length:(NSUInteger)iconLen] : nil;
    WISTrayOnMain(^{
        wisStatusMenuItem.title = [@"Status: " stringByAppendingString:statusValue];
        wisStatusItem.button.toolTip = [NSString stringWithFormat:@"%@ — %@", wisTrayLabel ?: @"wis-free-v3", statusValue];
        NSImage *image = [[NSImage alloc] initWithData:iconCopy];
        image.size = NSMakeSize(18, 18);
        image.template = templateIcon;
        wisStatusItem.button.image = image;
    });
}

void wis_tray_set_trigger_count(int count) {
    WISTrayOnMain(^{ wisCountMenuItem.title = [NSString stringWithFormat:@"Shortcut detected: %d times", count]; });
}

void wis_tray_set_startup(bool checked) {
    WISTrayOnMain(^{ wisStartupMenuItem.state = checked ? NSControlStateValueOn : NSControlStateValueOff; });
}

//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <ScreenCaptureKit/ScreenCaptureKit.h>

static NSDictionary *DKSTError(NSString *message) { return @{ @"error": message }; }

// Core Graphics Boolean is an unsigned byte. Boxing it directly produces a
// JSON number, whereas the Go boundary requires a JSON boolean.
static NSNumber *DKSTJSONBool(Boolean value) {
    return [NSNumber numberWithBool:(value != 0)];
}

static NSDictionary *DKSTScreenMetadata(CGRect bounds, pid_t pid, NSString *application, NSArray *windows, Boolean accessibility) {
    return @{ @"desktop_width": @(bounds.size.width), @"desktop_height": @(bounds.size.height),
        @"frontmost_pid": @(pid), @"application": application ?: @"", @"windows": windows,
        @"accessibility": DKSTJSONBool(accessibility) };
}

static NSDictionary *DKSTDisplayGeometry(CGDirectDisplayID display) {
    CGRect bounds = CGDisplayBounds(display);
    return @{ @"id": @(display), @"x": @(bounds.origin.x), @"y": @(bounds.origin.y),
        @"width": @(bounds.size.width), @"height": @(bounds.size.height), @"primary": DKSTJSONBool(display == CGMainDisplayID()) };
}

static NSDictionary *DKSTCaptureDisplay(CGDirectDisplayID display, NSString *path) {
    if (@available(macOS 14.0, *)) {
        // Load ScreenCaptureKit only on supported systems so older macOS
        // versions can still launch the Gateway and capture the primary screen.
        NSBundle *framework = [NSBundle bundleWithPath:@"/System/Library/Frameworks/ScreenCaptureKit.framework"];
        if (![framework load]) return DKSTError(@"Could not load ScreenCaptureKit.");
        Class contentClass = NSClassFromString(@"SCShareableContent");
        Class filterClass = NSClassFromString(@"SCContentFilter");
        Class configClass = NSClassFromString(@"SCStreamConfiguration");
        Class managerClass = NSClassFromString(@"SCScreenshotManager");
        if (!contentClass || !filterClass || !configClass || !managerClass) return DKSTError(@"Display capture is unavailable.");
        dispatch_semaphore_t done = dispatch_semaphore_create(0);
        __block NSDictionary *result;
        [contentClass getShareableContentExcludingDesktopWindows:NO onScreenWindowsOnly:YES
            completionHandler:^(SCShareableContent *content, NSError *error) {
                SCDisplay *selected = nil;
                for (SCDisplay *candidate in content.displays) {
                    if (candidate.displayID == display) { selected = candidate; break; }
                }
                if (!selected) {
                    result = DKSTError(error.localizedDescription ?: @"The selected display is unavailable for capture.");
                    dispatch_semaphore_signal(done);
                    return;
                }
                SCContentFilter *filter = [[filterClass alloc] initWithDisplay:selected excludingWindows:@[]];
                SCStreamConfiguration *config = [[configClass alloc] init];
                CGDisplayModeRef mode = CGDisplayCopyDisplayMode(display);
                config.width = mode ? CGDisplayModeGetPixelWidth(mode) : selected.width;
                config.height = mode ? CGDisplayModeGetPixelHeight(mode) : selected.height;
                if (mode) CGDisplayModeRelease(mode);
                config.showsCursor = NO;
                [managerClass captureImageWithFilter:filter configuration:config completionHandler:^(CGImageRef image, NSError *captureError) {
                    NSData *png = image ? [[[NSBitmapImageRep alloc] initWithCGImage:image] representationUsingType:NSBitmapImageFileTypePNG properties:@{}] : nil;
                    result = [png writeToFile:path atomically:YES] ? @{} : DKSTError(captureError.localizedDescription ?: @"Could not save the display capture.");
                    dispatch_semaphore_signal(done);
                }];
            }];
        if (dispatch_semaphore_wait(done, dispatch_time(DISPATCH_TIME_NOW, 8 * NSEC_PER_SEC)) != 0) {
            return DKSTError(@"Display capture timed out.");
        }
        return result;
    }
    return display == CGMainDisplayID() ? @{ @"legacy_primary": @YES } : DKSTError(@"Secondary display capture requires macOS 14 or later.");
}

static BOOL DKSTPostKey(CGKeyCode code, CGEventFlags flags, BOOL down) {
    CGEventRef event = CGEventCreateKeyboardEvent(NULL, code, down);
    if (!event) return NO;
    CGEventSetFlags(event, flags); CGEventPost(kCGHIDEventTap, event); CFRelease(event);
    return YES;
}

static NSDictionary *DKSTRun(NSString *action, NSDictionary *args) {
    NSWorkspace *workspace = NSWorkspace.sharedWorkspace;
    NSRunningApplication *front = workspace.frontmostApplication;
    if ([action isEqualToString:@"resolve_app"]) {
        NSString *application = args[@"application"];
        NSURL *url = [workspace URLForApplicationWithBundleIdentifier:application];
        return @{ @"application": url.path ?: application };
    }
    if ([action isEqualToString:@"list_apps"]) {
        NSMutableArray *apps = [NSMutableArray array];
        for (NSRunningApplication *app in workspace.runningApplications) {
            if (app.activationPolicy != NSApplicationActivationPolicyRegular || app.terminated) continue;
            [apps addObject:@{ @"pid": @(app.processIdentifier), @"name": app.localizedName ?: @"",
                @"bundle_id": app.bundleIdentifier ?: @"", @"path": app.bundleURL.path ?: @"",
                @"frontmost": DKSTJSONBool(app.processIdentifier == front.processIdentifier) }];
        }
        return @{ @"applications": apps, @"host": @"gateway macOS desktop" };
    }
    if ([action isEqualToString:@"observe"]) {
        if (!CGPreflightScreenCaptureAccess()) return DKSTError(@"Screen Recording permission is required. Enable it for the gateway app in System Settings > Privacy & Security > Screen Recording, then restart the app.");
        CGDirectDisplayID display = [args[@"display_id"] unsignedIntValue] ?: CGMainDisplayID();
        uint32_t count = 0;
        if (CGGetActiveDisplayList(0, NULL, &count) != kCGErrorSuccess || count == 0) return DKSTError(@"No active displays.");
        NSMutableData *ids = [NSMutableData dataWithLength:count * sizeof(CGDirectDisplayID)];
        if (CGGetActiveDisplayList(count, ids.mutableBytes, &count) != kCGErrorSuccess) return DKSTError(@"Could not list displays.");
        const CGDirectDisplayID *active = ids.bytes;
        NSMutableArray *displays = [NSMutableArray arrayWithCapacity:count];
        NSDictionary *selected = nil;
        for (uint32_t i = 0; i < count; i++) {
            NSDictionary *geometry = DKSTDisplayGeometry(active[i]);
            [displays addObject:geometry];
            if (active[i] == display) selected = geometry;
        }
        if (!selected) return DKSTError(@"The requested display is inactive; observe an active display.");
        CGRect bounds = CGDisplayBounds(display);
        NSMutableArray *windows = [NSMutableArray array];
        NSArray *info = CFBridgingRelease(CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID));
        if (!info) return DKSTError(@"macOS did not return a window list.");
        for (NSDictionary *window in info) {
            if ([window[(id)kCGWindowLayer] intValue] != 0 || [window[(id)kCGWindowAlpha] doubleValue] <= 0) continue;
            CGRect windowBounds;
            NSDictionary *windowMetadata = window[(id)kCGWindowBounds];
            if (!windowMetadata || !CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)windowMetadata, &windowBounds) ||
                !CGRectIntersectsRect(bounds, windowBounds)) continue;
            [windows addObject:@{ @"owner": window[(id)kCGWindowOwnerName] ?: @"", @"title": window[(id)kCGWindowName] ?: @"",
                @"pid": window[(id)kCGWindowOwnerPID] ?: @0, @"bounds": window[(id)kCGWindowBounds] ?: @{} }];
        }
        NSMutableDictionary *metadata = [DKSTScreenMetadata(bounds, front.processIdentifier, front.localizedName, windows, AXIsProcessTrusted()) mutableCopy];
        metadata[@"display"] = selected;
        metadata[@"displays"] = displays;
        return metadata;
    }
    if ([action isEqualToString:@"switch_app"]) {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:[args[@"pid"] intValue]];
        if (!app || app.terminated || app.activationPolicy != NSApplicationActivationPolicyRegular) return DKSTError(@"No switchable application with this pid. Call computer_list_apps again.");
        if (front.processIdentifier == app.processIdentifier) {
            return @{ @"status": @"already_frontmost", @"pid": @(app.processIdentifier) };
        }
        if (![app activateWithOptions:0]) {
            NSURL *bundle = app.bundleURL;
            if (!bundle.isFileURL || ![bundle.path.pathExtension.lowercaseString isEqualToString:@"app"]) {
                return DKSTError(@"Activation was rejected and the running app has no launchable bundle.");
            }
            return @{ @"status": @"activation_fallback", @"bundle_path": bundle.path };
        }
        return @{ @"status": @"activation_requested", @"pid": @(app.processIdentifier), @"next": @"Call computer_observe to verify focus before input." };
    }
    if (!AXIsProcessTrusted()) return DKSTError(@"Accessibility permission is required. Enable the gateway app in System Settings > Privacy & Security > Accessibility, then retry with a new observation.");
    if (!front || front.processIdentifier != [args[@"pid"] intValue]) return DKSTError(@"The foreground application changed. Call computer_observe before input.");
    NSDictionary *displayGeometry = args[@"display_geometry"];
    CGDirectDisplayID display = [displayGeometry[@"id"] unsignedIntValue];
    if (!display || !CGDisplayIsActive(display) || ![displayGeometry isEqualToDictionary:DKSTDisplayGeometry(display)]) {
        return DKSTError(@"The observed display changed. Call computer_observe again.");
    }
    CGRect displayBounds = CGDisplayBounds(display);
    if ([action isEqualToString:@"click"]) {
        CGPoint point = CGPointMake([args[@"x"] doubleValue], [args[@"y"] doubleValue]);
        if (!CGRectContainsPoint(displayBounds, point)) return DKSTError(@"Click is outside the observed display.");
        CGEventRef move = CGEventCreateMouseEvent(NULL, kCGEventMouseMoved, point, kCGMouseButtonLeft);
        if (!move) return DKSTError(@"Could not move the pointer.");
        CGEventSetFlags(move, 0); CGEventPost(kCGHIDEventTap, move); CFRelease(move);
        BOOL right = [args[@"button"] isEqualToString:@"right"];
        CGMouseButton button = right ? kCGMouseButtonRight : kCGMouseButtonLeft;
        NSInteger clicks = MAX(1, [args[@"clicks"] integerValue]);
        for (NSInteger n = 1; n <= clicks; n++) {
            CGEventRef down = CGEventCreateMouseEvent(NULL, right ? kCGEventRightMouseDown : kCGEventLeftMouseDown, point, button);
            CGEventRef up = CGEventCreateMouseEvent(NULL, right ? kCGEventRightMouseUp : kCGEventLeftMouseUp, point, button);
            if (!down || !up) { if (down) CFRelease(down); if (up) CFRelease(up); return DKSTError(@"Could not create mouse events."); }
            CGEventSetIntegerValueField(down, kCGMouseEventClickState, n);
            CGEventSetIntegerValueField(up, kCGMouseEventClickState, n);
            CGEventSetFlags(down, 0); CGEventSetFlags(up, 0);
            CGEventPost(kCGHIDEventTap, down); CGEventPost(kCGHIDEventTap, up);
            CFRelease(down); CFRelease(up);
        }
    } else if ([action isEqualToString:@"type"]) {
        NSString *text = args[@"text"];
        // Unicode events preserve Korean and other non-ASCII text without touching the clipboard.
        for (NSUInteger i = 0; i < text.length;) {
            NSRange range = [text rangeOfComposedCharacterSequenceAtIndex:i];
            NSString *part = [text substringWithRange:range];
            UniChar *chars = calloc(part.length, sizeof(UniChar));
            if (!chars) return DKSTError(@"Could not allocate Unicode input.");
            [part getCharacters:chars range:NSMakeRange(0, part.length)];
            for (int down = 1; down >= 0; down--) {
                CGEventRef event = CGEventCreateKeyboardEvent(NULL, 0, down);
                if (!event) { free(chars); return DKSTError(@"Could not create Unicode keyboard event."); }
                CGEventSetFlags(event, 0);
                CGEventKeyboardSetUnicodeString(event, part.length, chars);
                CGEventPost(kCGHIDEventTap, event); CFRelease(event);
            }
            free(chars); i = NSMaxRange(range);
        }
    } else if ([action isEqualToString:@"key"]) {
        CGEventFlags flags = 0;
        for (NSString *modifier in args[@"modifiers"]) {
            if ([modifier isEqualToString:@"command"]) flags |= kCGEventFlagMaskCommand;
            if ([modifier isEqualToString:@"control"]) flags |= kCGEventFlagMaskControl;
            if ([modifier isEqualToString:@"option"]) flags |= kCGEventFlagMaskAlternate;
            if ([modifier isEqualToString:@"shift"]) flags |= kCGEventFlagMaskShift;
        }
        CGKeyCode code = [args[@"key_code"] unsignedShortValue];
        BOOL down = DKSTPostKey(code, flags, YES);
        BOOL up = DKSTPostKey(code, flags, NO);
        if (!down || !up) return DKSTError(@"Could not create keyboard events.");
    } else if ([action isEqualToString:@"scroll"]) {
        NSString *direction = args[@"direction"];
        int amount = [args[@"amount"] intValue];
        int vertical = [direction isEqualToString:@"up"] ? amount : ([direction isEqualToString:@"down"] ? -amount : 0);
        int horizontal = [direction isEqualToString:@"left"] ? amount : ([direction isEqualToString:@"right"] ? -amount : 0);
        CGEventRef event = CGEventCreateScrollWheelEvent(NULL, kCGScrollEventUnitLine, 2, vertical, horizontal);
        if (!event) return DKSTError(@"Could not create scroll event.");
        CGEventRef pointer = CGEventCreate(NULL);
        if (!pointer) {
            CFRelease(event);
            return DKSTError(@"Could not read the current mouse position.");
        }
        CGPoint point = CGEventGetLocation(pointer);
        CFRelease(pointer);
        if (!CGRectContainsPoint(displayBounds, point)) {
            CFRelease(event);
            return DKSTError(@"Pointer is outside the observed display; observe the pointer's display before scrolling.");
        }
        CGEventSetLocation(event, point);
        CGEventPost(kCGHIDEventTap, event); CFRelease(event);
    } else return DKSTError(@"Unknown native computer action.");
    return @{ @"status": @"input_posted", @"next": @"Call computer_observe and verify the actual result before continuing." };
}

char *DKSTComputerExecute(const char *action, const char *arguments) {
    @autoreleasepool {
        NSString *name = [NSString stringWithUTF8String:action];
        NSDictionary *args = [NSJSONSerialization JSONObjectWithData:[[NSString stringWithUTF8String:arguments] dataUsingEncoding:NSUTF8StringEncoding] options:0 error:nil];
        __block NSDictionary *result;
        void (^run)(void) = ^{ @try { result = DKSTRun(name, args); } @catch (NSException *e) { result = DKSTError(e.reason ?: @"Native desktop error"); } };
        if ([name isEqualToString:@"capture_display"]) {
            result = DKSTCaptureDisplay([args[@"display_id"] unsignedIntValue], args[@"application"]);
        } else if (NSThread.isMainThread) {
            run();
        } else {
            dispatch_sync(dispatch_get_main_queue(), run);
        }
        NSData *data = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
        return strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
    }
}

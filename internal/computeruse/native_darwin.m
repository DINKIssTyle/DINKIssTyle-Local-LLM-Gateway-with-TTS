//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

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
        CGRect bounds = CGDisplayBounds(CGMainDisplayID());
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
        return DKSTScreenMetadata(bounds, front.processIdentifier, front.localizedName, windows, AXIsProcessTrusted());
    }
    if ([action isEqualToString:@"switch_app"]) {
        NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:[args[@"pid"] intValue]];
        if (!app || app.terminated || app.activationPolicy != NSApplicationActivationPolicyRegular) return DKSTError(@"No switchable application with this pid. Call computer_list_apps again.");
        if (![app activateWithOptions:0]) return DKSTError(@"Application activation was rejected.");
        return @{ @"status": @"activation_requested", @"pid": @(app.processIdentifier), @"next": @"Call computer_observe to verify focus before input." };
    }
    if (!AXIsProcessTrusted()) return DKSTError(@"Accessibility permission is required. Enable the gateway app in System Settings > Privacy & Security > Accessibility, then retry with a new observation.");
    if (!front || front.processIdentifier != [args[@"pid"] intValue]) return DKSTError(@"The foreground application changed. Call computer_observe before input.");
    if ([action isEqualToString:@"click"]) {
        CGPoint point = CGPointMake([args[@"x"] doubleValue], [args[@"y"] doubleValue]);
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
        if (NSThread.isMainThread) run(); else dispatch_sync(dispatch_get_main_queue(), run);
        NSData *data = [NSJSONSerialization dataWithJSONObject:result options:0 error:nil];
        return strdup([[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding].UTF8String);
    }
}

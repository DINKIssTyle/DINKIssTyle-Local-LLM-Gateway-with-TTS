#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

static BOOL DKSTHandleApplicationReopen(id delegate,
                                        SEL selector,
                                        NSApplication *application,
                                        BOOL hasVisibleWindows) {
    (void)selector;

    if (!hasVisibleWindows) {
        NSWindow *mainWindow = nil;
        @try {
            mainWindow = [delegate valueForKey:@"mainWindow"];
        } @catch (NSException *exception) {
            (void)exception;
        }

        if (mainWindow != nil) {
            if ([mainWindow isMiniaturized]) {
                [mainWindow deminiaturize:nil];
            }
            [mainWindow makeKeyAndOrderFront:nil];
        } else {
            for (NSWindow *window in application.windows) {
                [window makeKeyAndOrderFront:nil];
                break;
            }
        }
    }

    [application activateIgnoringOtherApps:YES];
    return YES;
}

static void DKSTInstallDockReopenHandlerOnMainThread(void *context) {
    (void)context;
    id delegate = NSApp.delegate;
    if (delegate == nil) {
        return;
    }

    SEL selector = @selector(applicationShouldHandleReopen:hasVisibleWindows:);
    Class delegateClass = [delegate class];
    if (class_getInstanceMethod(delegateClass, selector) == NULL) {
        class_addMethod(delegateClass,
                        selector,
                        (IMP)DKSTHandleApplicationReopen,
                        "c@:@c");
    }
}

void DKSTInstallDockReopenHandler(void) {
    if ([NSThread isMainThread]) {
        DKSTInstallDockReopenHandlerOnMainThread(NULL);
        return;
    }
    dispatch_async_f(dispatch_get_main_queue(), NULL, DKSTInstallDockReopenHandlerOnMainThread);
}

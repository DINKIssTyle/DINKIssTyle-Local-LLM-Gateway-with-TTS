#import <Cocoa/Cocoa.h>

extern void DKSTTrayShowMainWindow(void);
extern void DKSTTrayToggleServer(void);
extern void DKSTTrayQuit(void);

@interface DKSTStatusItemController : NSObject
@property(nonatomic, strong) NSStatusItem *statusItem;
@property(nonatomic, strong) NSMenuItem *serverStatusItem;
@property(nonatomic, strong) NSMenuItem *serverToggleItem;
@property(nonatomic, strong) NSMenuItem *showItem;
@property(nonatomic, strong) NSMenuItem *quitItem;
@end

@implementation DKSTStatusItemController

- (instancetype)initWithIconData:(NSData *)iconData {
    self = [super init];
    if (self == nil) {
        return nil;
    }

    self.statusItem = [[NSStatusBar systemStatusBar]
        statusItemWithLength:NSSquareStatusItemLength];
    self.statusItem.button.toolTip = @"DKST LLM Chat Server";
    self.statusItem.button.accessibilityLabel = @"DKST LLM Chat Server";

    if (iconData.length > 0) {
        NSImage *image = [[NSImage alloc] initWithData:iconData];
        if (image != nil) {
            image.size = NSMakeSize(18.0, 18.0);
            image.template = YES;
            self.statusItem.button.image = image;
            self.statusItem.button.imagePosition = NSImageOnly;
        }
    }
    if (self.statusItem.button.image == nil) {
        self.statusItem.button.title = @"DKST";
    }

    NSMenu *menu = [[NSMenu alloc] initWithTitle:@"DKST LLM Chat Server"];
    menu.autoenablesItems = NO;

    self.serverStatusItem = [[NSMenuItem alloc]
        initWithTitle:@"서버 상태: 중지됨" action:nil keyEquivalent:@""];
    self.serverStatusItem.enabled = NO;
    [menu addItem:self.serverStatusItem];

    self.serverToggleItem = [[NSMenuItem alloc]
        initWithTitle:@"서버 시작"
               action:@selector(toggleServer:)
        keyEquivalent:@""];
    self.serverToggleItem.target = self;
    [menu addItem:self.serverToggleItem];

    [menu addItem:[NSMenuItem separatorItem]];

    self.showItem = [[NSMenuItem alloc]
        initWithTitle:@"메인 창 열기"
               action:@selector(showMainWindow:)
        keyEquivalent:@""];
    self.showItem.target = self;
    [menu addItem:self.showItem];

    [menu addItem:[NSMenuItem separatorItem]];

    self.quitItem = [[NSMenuItem alloc]
        initWithTitle:@"종료" action:@selector(quit:) keyEquivalent:@""];
    self.quitItem.target = self;
    [menu addItem:self.quitItem];

    self.statusItem.menu = menu;
    return self;
}

- (void)setStatusTitle:(NSString *)statusTitle
           toggleTitle:(NSString *)toggleTitle
             showTitle:(NSString *)showTitle
             quitTitle:(NSString *)quitTitle {
    self.serverStatusItem.title = statusTitle;
    self.serverToggleItem.title = toggleTitle;
    self.showItem.title = showTitle;
    self.quitItem.title = quitTitle;
}

- (void)showMainWindow:(id)sender {
    (void)sender;
    DKSTTrayShowMainWindow();
}

- (void)toggleServer:(id)sender {
    (void)sender;
    DKSTTrayToggleServer();
}

- (void)quit:(id)sender {
    (void)sender;
    DKSTTrayQuit();
}

@end

static DKSTStatusItemController *DKSTStatusController = nil;
static NSString *DKSTPendingStatusTitle = @"서버 상태: 중지됨";
static NSString *DKSTPendingToggleTitle = @"서버 시작";
static NSString *DKSTPendingShowTitle = @"메인 창 열기";
static NSString *DKSTPendingQuitTitle = @"종료";

static void DKSTOnMainThread(dispatch_block_t block, BOOL wait) {
    if ([NSThread isMainThread]) {
        block();
        return;
    }
    if (wait) {
        dispatch_sync(dispatch_get_main_queue(), block);
    } else {
        dispatch_async(dispatch_get_main_queue(), block);
    }
}

void DKSTInitStatusItem(const unsigned char *iconBytes, int iconLength) {
    NSData *iconData = nil;
    if (iconBytes != NULL && iconLength > 0) {
        iconData = [NSData dataWithBytes:iconBytes length:(NSUInteger)iconLength];
    }

    DKSTOnMainThread(^{
        if (DKSTStatusController == nil) {
            DKSTStatusController = [[DKSTStatusItemController alloc]
                initWithIconData:iconData];
        }
        [DKSTStatusController setStatusTitle:DKSTPendingStatusTitle
                                 toggleTitle:DKSTPendingToggleTitle
                                   showTitle:DKSTPendingShowTitle
                                   quitTitle:DKSTPendingQuitTitle];
    }, YES);
}

void DKSTUpdateStatusItem(const char *statusTitle, const char *toggleTitle,
                          const char *showTitle, const char *quitTitle) {
    NSString *status = [NSString stringWithUTF8String:statusTitle];
    NSString *toggle = [NSString stringWithUTF8String:toggleTitle];
    NSString *show = [NSString stringWithUTF8String:showTitle];
    NSString *quit = [NSString stringWithUTF8String:quitTitle];
    DKSTOnMainThread(^{
        DKSTPendingStatusTitle = status;
        DKSTPendingToggleTitle = toggle;
        DKSTPendingShowTitle = show;
        DKSTPendingQuitTitle = quit;
        [DKSTStatusController setStatusTitle:DKSTPendingStatusTitle
                                 toggleTitle:DKSTPendingToggleTitle
                                   showTitle:DKSTPendingShowTitle
                                   quitTitle:DKSTPendingQuitTitle];
    }, NO);
}

void DKSTRemoveStatusItem(void) {
    DKSTOnMainThread(^{
        if (DKSTStatusController != nil) {
            [[NSStatusBar systemStatusBar]
                removeStatusItem:DKSTStatusController.statusItem];
            DKSTStatusController = nil;
        }
    }, NO);
}

void DKSTSetDockIconVisible(int visible) {
    DKSTOnMainThread(^{
        NSApplicationActivationPolicy policy = visible != 0
            ? NSApplicationActivationPolicyRegular
            : NSApplicationActivationPolicyAccessory;
        [NSApp setActivationPolicy:policy];
        if (visible != 0) {
            [NSApp activateIgnoringOtherApps:YES];
        }
    }, YES);
}

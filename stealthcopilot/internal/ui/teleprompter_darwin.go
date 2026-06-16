//go:build darwin && cgo

package ui

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework Cocoa
#include <Cocoa/Cocoa.h>
#include <stdlib.h>

@interface SCTeleprompterBridge : NSObject
+ (void)show;
+ (void)hide;
+ (void)toggleMinimized;
+ (void)closeWindow;
+ (void)setAppearanceWithFontSize:(NSNumber *)fontSize opacity:(NSNumber *)opacity;
+ (void)appendSubtitle:(NSString *)text;
+ (void)appendAnswerToken:(NSString *)text;
+ (void)finishAnswer;
+ (void)setError:(NSString *)message;
+ (void)setCircuitOpen:(NSNumber *)open;
+ (void)reset;
@end

static NSWindow *scTeleprompterWindow = nil;
static NSView *scContentView = nil;
static NSView *scExpandedView = nil;
static NSButton *scPillButton = nil;
static NSTextField *scErrorLabel = nil;
static NSTextField *scCircuitLabel = nil;
static NSTextView *scSubtitleView = nil;
static NSTextView *scAnswerView = nil;
static NSMutableString *scSubtitleText = nil;
static NSMutableString *scAnswerText = nil;
static BOOL scAnswering = NO;
static BOOL scMinimized = NO;
static CGFloat scFontSize = 16.0;
static CGFloat scOpacity = 0.86;

static NSTextField *scMakeLabel(NSRect frame, NSString *text, NSColor *color) {
	NSTextField *label = [[NSTextField alloc] initWithFrame:frame];
	[label setEditable:NO];
	[label setSelectable:NO];
	[label setBezeled:NO];
	[label setDrawsBackground:NO];
	[label setStringValue:text ? text : @""];
	[label setTextColor:color];
	[label setFont:[NSFont systemFontOfSize:12 weight:NSFontWeightMedium]];
	return label;
}

static NSButton *scMakeButton(NSRect frame, NSString *title, SEL action) {
	NSButton *button = [[NSButton alloc] initWithFrame:frame];
	[button setTitle:title];
	[button setTarget:[SCTeleprompterBridge class]];
	[button setAction:action];
	[button setBezelStyle:NSBezelStyleTexturedRounded];
	[button setBordered:NO];
	[button setFont:[NSFont systemFontOfSize:13 weight:NSFontWeightSemibold]];
	[button setContentTintColor:[NSColor colorWithCalibratedWhite:0.88 alpha:1.0]];
	return button;
}

static void scApplyLayerAppearance(void) {
	if (scContentView != nil && [scContentView layer] != nil) {
		[[scContentView layer] setBackgroundColor:[[NSColor colorWithCalibratedRed:0.07 green:0.09 blue:0.14 alpha:scOpacity] CGColor]];
	}
	NSFont *font = [NSFont systemFontOfSize:scFontSize weight:NSFontWeightRegular];
	[scSubtitleView setFont:font];
	[scAnswerView setFont:font];
}

static NSTextView *scMakeTextView(NSRect frame) {
	NSTextView *view = [[NSTextView alloc] initWithFrame:frame];
	[view setEditable:NO];
	[view setSelectable:NO];
	[view setDrawsBackground:NO];
	[view setTextColor:[NSColor colorWithCalibratedRed:0.90 green:0.98 blue:1.00 alpha:1.0]];
	[view setFont:[NSFont systemFontOfSize:scFontSize weight:NSFontWeightRegular]];
	[view setTextContainerInset:NSMakeSize(12, 10)];
	[[view textContainer] setWidthTracksTextView:YES];
	[[view textContainer] setHeightTracksTextView:NO];
	return view;
}

static NSScrollView *scMakeScrollView(NSRect frame, NSTextView **textView) {
	NSScrollView *scroll = [[NSScrollView alloc] initWithFrame:frame];
	[scroll setBorderType:NSNoBorder];
	[scroll setHasVerticalScroller:YES];
	[scroll setDrawsBackground:NO];
	*textView = scMakeTextView(NSMakeRect(0, 0, frame.size.width, frame.size.height));
	[scroll setDocumentView:*textView];
	return scroll;
}

static void scEnsureWindow(void) {
	if (scTeleprompterWindow != nil) {
		return;
	}

	NSRect frame = NSMakeRect(0, 0, 420, 320);
	scTeleprompterWindow = [[NSWindow alloc]
		initWithContentRect:frame
		styleMask:NSWindowStyleMaskBorderless
		backing:NSBackingStoreBuffered
		defer:NO];
	[scTeleprompterWindow setTitle:@"StealthCopilot"];
	[scTeleprompterWindow setLevel:NSFloatingWindowLevel];
	[scTeleprompterWindow setOpaque:NO];
	[scTeleprompterWindow setBackgroundColor:[NSColor clearColor]];
	[scTeleprompterWindow setHasShadow:YES];
	[scTeleprompterWindow setReleasedWhenClosed:NO];
	[scTeleprompterWindow setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces | NSWindowCollectionBehaviorFullScreenAuxiliary];
	[scTeleprompterWindow setSharingType:NSWindowSharingNone];

	scContentView = [[NSView alloc] initWithFrame:frame];
	[scContentView setWantsLayer:YES];
	[[scContentView layer] setBackgroundColor:[[NSColor colorWithCalibratedRed:0.07 green:0.09 blue:0.14 alpha:scOpacity] CGColor]];
	[[scContentView layer] setCornerRadius:10.0];
	[[scContentView layer] setMasksToBounds:YES];

	scExpandedView = [[NSView alloc] initWithFrame:frame];
	NSTextField *title = scMakeLabel(NSMakeRect(14, 287, 220, 22), @"StealthCopilot", [NSColor colorWithCalibratedRed:0.78 green:0.92 blue:1.0 alpha:1.0]);
	[scExpandedView addSubview:title];
	[scExpandedView addSubview:scMakeButton(NSMakeRect(354, 286, 28, 24), @"–", @selector(toggleMinimized))];
	[scExpandedView addSubview:scMakeButton(NSMakeRect(386, 286, 28, 24), @"×", @selector(closeWindow))];

	scErrorLabel = scMakeLabel(NSMakeRect(12, 262, 396, 20), @"", [NSColor colorWithCalibratedRed:1.0 green:0.72 blue:0.72 alpha:1.0]);
	scCircuitLabel = scMakeLabel(NSMakeRect(12, 240, 396, 20), @"", [NSColor colorWithCalibratedRed:1.0 green:0.77 blue:0.36 alpha:1.0]);
	[scExpandedView addSubview:scErrorLabel];
	[scExpandedView addSubview:scCircuitLabel];

	NSScrollView *subtitleScroll = scMakeScrollView(NSMakeRect(0, 136, 420, 102), &scSubtitleView);
	NSScrollView *answerScroll = scMakeScrollView(NSMakeRect(0, 0, 420, 134), &scAnswerView);
	[scExpandedView addSubview:subtitleScroll];
	[scExpandedView addSubview:answerScroll];

	NSView *divider = [[NSView alloc] initWithFrame:NSMakeRect(0, 134, 420, 1)];
	[divider setWantsLayer:YES];
	[[divider layer] setBackgroundColor:[[NSColor colorWithCalibratedWhite:1.0 alpha:0.16] CGColor]];
	[scExpandedView addSubview:divider];

	scPillButton = scMakeButton(NSMakeRect(0, 0, 190, 38), @"StealthCopilot", @selector(toggleMinimized));
	[scPillButton setHidden:YES];
	[scContentView addSubview:scExpandedView];
	[scContentView addSubview:scPillButton];

	[scTeleprompterWindow setContentView:scContentView];
	[scTeleprompterWindow center];

	scSubtitleText = [[NSMutableString alloc] init];
	scAnswerText = [[NSMutableString alloc] init];
}

static void scScrollToBottom(NSTextView *view) {
	NSRange range = NSMakeRange([[view string] length], 0);
	[view scrollRangeToVisible:range];
}

@implementation SCTeleprompterBridge
+ (void)show {
	scEnsureWindow();
	[scTeleprompterWindow makeKeyAndOrderFront:nil];
	[scTeleprompterWindow orderFrontRegardless];
}

	+ (void)hide {
		if (scTeleprompterWindow != nil) {
			[scTeleprompterWindow orderOut:nil];
		}
	}

	+ (void)toggleMinimized {
		scEnsureWindow();
		scMinimized = !scMinimized;
		[scExpandedView setHidden:scMinimized];
		[scPillButton setHidden:!scMinimized];
		if (scMinimized) {
			[scTeleprompterWindow setFrame:NSMakeRect([scTeleprompterWindow frame].origin.x, [scTeleprompterWindow frame].origin.y, 190, 38) display:YES animate:YES];
		} else {
			[scTeleprompterWindow setFrame:NSMakeRect([scTeleprompterWindow frame].origin.x, [scTeleprompterWindow frame].origin.y, 420, 320) display:YES animate:YES];
		}
	}

	+ (void)closeWindow {
		[self hide];
	}

	+ (void)setAppearanceWithFontSize:(NSNumber *)fontSize opacity:(NSNumber *)opacity {
		scEnsureWindow();
		scFontSize = [fontSize doubleValue];
		if (scFontSize < 13.0) scFontSize = 13.0;
		if (scFontSize > 28.0) scFontSize = 28.0;
		scOpacity = [opacity doubleValue];
		if (scOpacity < 0.3) scOpacity = 0.3;
		if (scOpacity > 1.0) scOpacity = 1.0;
		scApplyLayerAppearance();
	}

+ (void)appendSubtitle:(NSString *)text {
	scEnsureWindow();
	if ([text length] == 0) {
		return;
	}
	if ([scSubtitleText length] > 0) {
		[scSubtitleText appendString:@"\n\n"];
	}
	[scSubtitleText appendString:text];
	[scSubtitleView setString:scSubtitleText];
	scScrollToBottom(scSubtitleView);
}

+ (void)appendAnswerToken:(NSString *)text {
	scEnsureWindow();
	if ([text length] == 0) {
		return;
	}
	scAnswering = YES;
	[scAnswerText appendString:text];
	[scAnswerView setString:scAnswerText];
	scScrollToBottom(scAnswerView);
}

	+ (void)finishAnswer {
		scAnswering = NO;
	}

	+ (void)setError:(NSString *)message {
		scEnsureWindow();
		[scErrorLabel setStringValue:message ? message : @""];
	}

	+ (void)setCircuitOpen:(NSNumber *)open {
		scEnsureWindow();
		BOOL isOpen = [open boolValue];
		[scCircuitLabel setStringValue:isOpen ? @"云端管道已断开，当前为本地直通模式" : @""];
	}

	+ (void)reset {
		scEnsureWindow();
		[scSubtitleText setString:@""];
		[scAnswerText setString:@""];
		[scSubtitleView setString:@""];
		[scAnswerView setString:@""];
		[scErrorLabel setStringValue:@""];
		[scCircuitLabel setStringValue:@""];
		scAnswering = NO;
	}
	@end

static void scShowTeleprompter(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(show) withObject:nil waitUntilDone:NO];
}

static void scHideTeleprompter(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(hide) withObject:nil waitUntilDone:NO];
}

static void scSetAppearance(double fontSize, double opacity) {
	NSNumber *f = [NSNumber numberWithDouble:fontSize];
	NSNumber *o = [NSNumber numberWithDouble:opacity];
	NSArray *args = @[f, o];
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(scApplyAppearanceArray:) withObject:args waitUntilDone:NO];
}

@interface SCTeleprompterBridge (AppearanceArray)
+ (void)scApplyAppearanceArray:(NSArray *)args;
@end

@implementation SCTeleprompterBridge (AppearanceArray)
+ (void)scApplyAppearanceArray:(NSArray *)args {
	[SCTeleprompterBridge setAppearanceWithFontSize:[args objectAtIndex:0] opacity:[args objectAtIndex:1]];
}
@end

static void scAppendSubtitle(const char *text) {
	NSString *s = [[NSString alloc] initWithUTF8String:text ? text : ""];
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(appendSubtitle:) withObject:s waitUntilDone:NO];
}

static void scAppendAnswerToken(const char *text) {
	NSString *s = [[NSString alloc] initWithUTF8String:text ? text : ""];
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(appendAnswerToken:) withObject:s waitUntilDone:NO];
}

static void scFinishAnswer(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(finishAnswer) withObject:nil waitUntilDone:NO];
}

static void scSetError(const char *text) {
	NSString *s = [[NSString alloc] initWithUTF8String:text ? text : ""];
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(setError:) withObject:s waitUntilDone:NO];
}

static void scSetCircuitOpen(int open) {
	NSNumber *n = [NSNumber numberWithBool:(open != 0)];
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(setCircuitOpen:) withObject:n waitUntilDone:NO];
}

static void scResetTeleprompter(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(reset) withObject:nil waitUntilDone:NO];
}
*/
import "C"

import "unsafe"

type darwinTeleprompterWindow struct{}

func newPlatformTeleprompterWindow() TeleprompterWindow {
	return &darwinTeleprompterWindow{}
}

func (w *darwinTeleprompterWindow) Show() error {
	C.scShowTeleprompter()
	return nil
}

func (w *darwinTeleprompterWindow) Hide() error {
	C.scHideTeleprompter()
	return nil
}

func (w *darwinTeleprompterWindow) SetAppearance(fontSize int, opacity float64) {
	C.scSetAppearance(C.double(fontSize), C.double(opacity))
}

func (w *darwinTeleprompterWindow) AppendSubtitle(text string) {
	cText := C.CString(text)
	defer C.free(unsafe.Pointer(cText))
	C.scAppendSubtitle(cText)
}

func (w *darwinTeleprompterWindow) AppendAnswerToken(token string) {
	cText := C.CString(token)
	defer C.free(unsafe.Pointer(cText))
	C.scAppendAnswerToken(cText)
}

func (w *darwinTeleprompterWindow) FinishAnswer() {
	C.scFinishAnswer()
}

func (w *darwinTeleprompterWindow) SetError(message string) {
	cText := C.CString(message)
	defer C.free(unsafe.Pointer(cText))
	C.scSetError(cText)
}

func (w *darwinTeleprompterWindow) SetCircuitOpen(open bool) {
	if open {
		C.scSetCircuitOpen(1)
		return
	}
	C.scSetCircuitOpen(0)
}

func (w *darwinTeleprompterWindow) Reset() {
	C.scResetTeleprompter()
}

func (w *darwinTeleprompterWindow) Close() error {
	C.scHideTeleprompter()
	return nil
}

func (w *darwinTeleprompterWindow) Available() bool {
	return true
}

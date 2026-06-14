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
+ (void)appendSubtitle:(NSString *)text;
+ (void)appendAnswerToken:(NSString *)text;
+ (void)finishAnswer;
@end

static NSWindow *scTeleprompterWindow = nil;
static NSTextView *scSubtitleView = nil;
static NSTextView *scAnswerView = nil;
static NSMutableString *scSubtitleText = nil;
static NSMutableString *scAnswerText = nil;
static BOOL scAnswering = NO;

static NSTextView *scMakeTextView(NSRect frame) {
	NSTextView *view = [[NSTextView alloc] initWithFrame:frame];
	[view setEditable:NO];
	[view setSelectable:NO];
	[view setDrawsBackground:NO];
	[view setTextColor:[NSColor colorWithCalibratedRed:0.90 green:0.98 blue:1.00 alpha:1.0]];
	[view setFont:[NSFont systemFontOfSize:16 weight:NSFontWeightRegular]];
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

	NSView *content = [[NSView alloc] initWithFrame:frame];
	[content setWantsLayer:YES];
	[[content layer] setBackgroundColor:[[NSColor colorWithCalibratedRed:0.07 green:0.09 blue:0.14 alpha:0.86] CGColor]];
	[[content layer] setCornerRadius:10.0];
	[[content layer] setMasksToBounds:YES];

	NSScrollView *subtitleScroll = scMakeScrollView(NSMakeRect(0, 160, 420, 160), &scSubtitleView);
	NSScrollView *answerScroll = scMakeScrollView(NSMakeRect(0, 0, 420, 158), &scAnswerView);
	[content addSubview:subtitleScroll];
	[content addSubview:answerScroll];

	NSView *divider = [[NSView alloc] initWithFrame:NSMakeRect(0, 158, 420, 1)];
	[divider setWantsLayer:YES];
	[[divider layer] setBackgroundColor:[[NSColor colorWithCalibratedWhite:1.0 alpha:0.16] CGColor]];
	[content addSubview:divider];

	[scTeleprompterWindow setContentView:content];
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
@end

static void scShowTeleprompter(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(show) withObject:nil waitUntilDone:NO];
}

static void scHideTeleprompter(void) {
	[SCTeleprompterBridge performSelectorOnMainThread:@selector(hide) withObject:nil waitUntilDone:NO];
}

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

func (w *darwinTeleprompterWindow) Close() error {
	C.scHideTeleprompter()
	return nil
}

func (w *darwinTeleprompterWindow) Available() bool {
	return true
}

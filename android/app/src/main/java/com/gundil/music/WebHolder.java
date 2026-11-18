package com.gundil.music;

import android.content.MutableContextWrapper;
import android.os.Handler;
import android.os.Looper;
import android.view.ViewGroup;
import android.webkit.WebView;

/**
 * Holds the app's one WebView, where the music plays. It belongs to the
 * process rather than to the screen: when the app is swiped away from
 * Recents while music plays, the screen (MainActivity) goes but the WebView
 * stays, kept running by PlaybackService. Opening the app again shows the
 * same page, still playing.
 */
final class WebHolder {
    static WebView web;
    /** The WebView's context: the open screen, or the application while none is open. */
    static MutableContextWrapper context;
    static final Handler ui = new Handler(Looper.getMainLooper());

    private WebHolder() { }

    /** Runs JavaScript in the page (from any thread). */
    static void js(String code) {
        ui.post(() -> {
            if (web != null) web.evaluateJavascript(code, null);
        });
    }

    /** Takes the WebView off the screen without stopping it. */
    static void detach() {
        if (web == null) return;
        ViewGroup parent = (ViewGroup) web.getParent();
        if (parent != null) parent.removeView(web);
        if (context != null) context.setBaseContext(web.getContext().getApplicationContext());
    }

    /** Stops the page for good (nothing is playing, or the renderer is gone). */
    static void destroy() {
        if (web == null) return;
        WebView w = web;
        web = null;
        ViewGroup parent = (ViewGroup) w.getParent();
        if (parent != null) parent.removeView(w);
        w.destroy();
    }
}

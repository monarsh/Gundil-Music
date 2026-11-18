package com.gundil.music;

import android.Manifest;
import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.Context;
import android.content.Intent;
import android.content.MutableContextWrapper;
import android.content.SharedPreferences;
import android.content.pm.PackageManager;
import android.graphics.Color;
import android.net.ConnectivityManager;
import android.net.LinkAddress;
import android.net.LinkProperties;
import android.net.Network;
import android.net.Uri;
import android.os.Build;
import android.os.Bundle;
import android.os.Handler;
import android.os.Looper;
import android.text.InputType;
import android.view.View;
import android.view.ViewGroup;
import android.view.WindowInsets;
import android.webkit.CookieManager;
import android.webkit.HttpAuthHandler;
import android.webkit.JavascriptInterface;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
import android.widget.FrameLayout;
import android.widget.LinearLayout;

import org.json.JSONArray;
import org.json.JSONObject;

import java.io.ByteArrayOutputStream;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.Inet4Address;
import java.net.InetAddress;
import java.net.URL;
import java.util.ArrayList;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.TimeUnit;

/**
 * Gundil Music for Android: shows the app served by your Gundil Music server
 * (a Raspberry Pi, Orange Pi or PC on your network) and keeps playing in the
 * background with a media notification, lock-screen and headset controls.
 */
public class MainActivity extends Activity {
    static final int PORT = 8338;
    private static final String SETUP_URL = "file:///android_asset/setup.html";

    /** The open screen, if any (music can play with none: see WebHolder). */
    static MainActivity instance;
    private static final ExecutorService pool = Executors.newCachedThreadPool();
    private static boolean authAsked;

    private WebView web;
    private SharedPreferences prefs;
    private final Handler ui = new Handler(Looper.getMainLooper());

    static SharedPreferences prefs(Context c) {
        return c.getApplicationContext().getSharedPreferences("gundil", MODE_PRIVATE);
    }

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        instance = this;
        prefs = prefs(this);

        if ((getApplicationInfo().flags & android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true); // debug builds only, for automated tests
        }
        // "adb shell am start ... --es server 192.168.1.20" sets the server directly.
        String given = getIntent() != null ? getIntent().getStringExtra("server") : null;
        if (given != null && !normalize(given).isEmpty()) prefs.edit().putString("server", normalize(given)).apply();

        boolean reuse = WebHolder.web != null;
        if (reuse) {
            // Music kept playing after the app was swiped away: show the same page.
            web = WebHolder.web;
            WebHolder.detach();
            WebHolder.context.setBaseContext(this);
        } else {
            WebHolder.context = new MutableContextWrapper(this);
            web = new KeepPlayingWebView(WebHolder.context);
            WebHolder.web = web;
            setUp(web, getApplicationContext());
        }
        web.setLayoutParams(new ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        FrameLayout root = new FrameLayout(this);
        root.setBackgroundColor(Color.parseColor("#030303"));
        root.addView(web);
        keepClearOfSystemBars(root);
        setContentView(root);

        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1);
        }

        if (reuse) {
            // Already showing the app.
        } else if (state != null) {
            web.restoreState(state);
        } else {
            String server = prefs.getString("server", "");
            if (server.isEmpty()) showSetup(null);
            else web.loadUrl(server + "/");
        }
    }

    /**
     * Android 15 draws apps under the status bar, the navigation or gesture
     * bar and the camera cutout. Keep the page clear of them (and of the
     * keyboard, which edge-to-edge apps must also make room for themselves).
     */
    private static void keepClearOfSystemBars(View root) {
        root.setOnApplyWindowInsetsListener((v, insets) -> {
            if (Build.VERSION.SDK_INT >= 30) {
                android.graphics.Insets bars = insets.getInsets(WindowInsets.Type.systemBars() | WindowInsets.Type.displayCutout());
                android.graphics.Insets ime = insets.getInsets(WindowInsets.Type.ime());
                v.setPadding(bars.left, bars.top, bars.right, Math.max(bars.bottom, ime.bottom));
                return WindowInsets.CONSUMED;
            }
            v.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(),
                    insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
            return insets.consumeSystemWindowInsets();
        });
        root.requestApplyInsets();
    }

    @Override
    protected void onSaveInstanceState(Bundle out) {
        super.onSaveInstanceState(out);
        web.saveState(out);
    }

    // Deliberately no web.onPause(): the page must keep playing in the background.
    // The page is told instead, so it can stop the music video's picture (not the sound).

    @Override
    protected void onStart() {
        super.onStart();
        if (web != null) js("window.gundilVisible && window.gundilVisible(true)");
    }

    @Override
    protected void onStop() {
        if (web != null) js("window.gundilVisible && window.gundilVisible(false)");
        CookieManager.getInstance().flush(); // stay signed in to the server
        super.onStop();
    }

    @Override
    protected void onDestroy() {
        if (instance == this) instance = null;
        if (WebHolder.web != null && (isChangingConfigurations() || PlaybackService.playingHere())) {
            // Swiped away from Recents (or rotated) while music plays on this
            // phone: keep the page and the music going without a screen.
            WebHolder.detach();
        } else {
            stopService(new Intent(this, PlaybackService.class));
            WebHolder.destroy();
        }
        super.onDestroy();
    }

    /** Sets up a new WebView: settings, the page's bridge and its client. */
    @SuppressLint("SetJavaScriptEnabled")
    private static void setUp(WebView web, Context app) {
        web.setBackgroundColor(Color.parseColor("#030303"));
        WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setMediaPlaybackRequiresUserGesture(false);
        ws.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        ws.setUserAgentString(ws.getUserAgentString() + " GundilMusicAndroid/" + BuildConfigVersion.name(app));
        if (Build.VERSION.SDK_INT >= 26) {
            // Keep the page's renderer as important as the app, even without a screen.
            web.setRendererPriorityPolicy(WebView.RENDERER_PRIORITY_IMPORTANT, false);
        }
        web.addJavascriptInterface(new Bridge(app), "GundilAndroid");
        web.setWebChromeClient(new WebChromeClient());
        web.setWebViewClient(new Client(app));
    }

    /** Handles the page's links, sign-in prompts and errors. Holds no screen. */
    static final class Client extends WebViewClient {
        private final Context app;

        Client(Context app) { this.app = app; }

        @Override
        public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest req) {
            Uri u = req.getUrl();
            String server = prefs(app).getString("server", "");
            if (u.getScheme() != null && u.getScheme().startsWith("http") && !server.isEmpty() && u.toString().startsWith(server)) {
                return false;
            }
            if ("file".equals(u.getScheme())) return false;
            // Links that leave the server (e.g. "Share" links) open in the browser.
            try { app.startActivity(new Intent(Intent.ACTION_VIEW, u).addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)); } catch (Exception ignored) { }
            return true;
        }

        @Override
        public void onReceivedHttpAuthRequest(WebView view, HttpAuthHandler handler, String host, String realm) {
            SharedPreferences prefs = prefs(app);
            String user = prefs.getString("user", ""), pass = prefs.getString("pass", "");
            if (!authAsked && !user.isEmpty()) {
                authAsked = true;
                handler.proceed(user, pass);
                return;
            }
            MainActivity a = instance;
            if (a != null) a.askLogin(handler);
            else handler.cancel();
        }

        @Override
        public WebResourceResponse shouldInterceptRequest(WebView view, WebResourceRequest req) {
            // The app's own files come with a saved copy, for opening it with the server off.
            return Shell.serve(app, prefs(app).getString("server", ""), req);
        }

        @Override
        public void onPageFinished(WebView view, String url) {
            if (url.startsWith("http")) authAsked = false;
        }

        @Override
        public boolean onRenderProcessGone(WebView view, android.webkit.RenderProcessGoneDetail detail) {
            // The page's renderer crashed or was killed: start over instead of letting Android close the app.
            android.util.Log.w("GundilMusic", "WebView renderer gone (crash=" + detail.didCrash() + "), reloading");
            WebHolder.destroy();
            MainActivity a = instance;
            if (a != null) a.recreate();
            else app.stopService(new Intent(app, PlaybackService.class));
            return true;
        }

        @Override
        public void onReceivedError(WebView view, WebResourceRequest req, WebResourceError err) {
            if (req.isForMainFrame() && !req.getUrl().toString().startsWith("file:")) {
                showSetup("Can't reach " + prefs(app).getString("server", "the server") + ". Check that it is running and that this phone is on the same network.");
            }
        }
    }

    @Override
    public void onBackPressed() {
        // Let the web app close its player or menus first.
        web.evaluateJavascript("window.gundilBack ? window.gundilBack() : false", value -> {
            if ("true".equals(value)) return;
            if (web.canGoBack()) web.goBack();
            else moveTaskToBack(true);
        });
    }

    static void showSetup(String error) {
        String url = SETUP_URL + (error != null ? "#error=" + Uri.encode(error) : "");
        if (WebHolder.web != null) WebHolder.web.loadUrl(url);
    }

    private void askLogin(HttpAuthHandler handler) {
        LinearLayout box = new LinearLayout(this);
        box.setOrientation(LinearLayout.VERTICAL);
        int pad = (int) (20 * getResources().getDisplayMetrics().density);
        box.setPadding(pad, pad / 2, pad, 0);
        EditText user = new EditText(this);
        user.setHint("User name");
        user.setText(prefs.getString("user", ""));
        EditText pass = new EditText(this);
        pass.setHint("Password");
        pass.setInputType(InputType.TYPE_CLASS_TEXT | InputType.TYPE_TEXT_VARIATION_PASSWORD);
        box.addView(user);
        box.addView(pass);
        new AlertDialog.Builder(this)
                .setTitle("Sign in to Gundil Music")
                .setView(box)
                .setCancelable(false)
                .setPositiveButton("Sign in", (d, w) -> {
                    prefs.edit().putString("user", user.getText().toString()).putString("pass", pass.getText().toString()).apply();
                    authAsked = true;
                    handler.proceed(user.getText().toString(), pass.getText().toString());
                })
                .setNegativeButton("Change server", (d, w) -> {
                    handler.cancel();
                    showSetup(null);
                })
                .show();
    }

    /** Runs a command in the web app, e.g. from the notification or headset buttons. */
    static void control(String cmd) {
        WebHolder.js("window.gundilControl && window.gundilControl('" + cmd + "')");
    }

    // ---------- talking to servers ----------

    static String normalize(String input) {
        String s = input.trim();
        if (s.isEmpty()) return "";
        if (!s.startsWith("http://") && !s.startsWith("https://")) s = "http://" + s;
        Uri u = Uri.parse(s);
        String host = u.getHost();
        if (host == null || host.isEmpty()) return "";
        int port = u.getPort() > 0 ? u.getPort() : (s.startsWith("https://") ? 443 : PORT);
        boolean defaultPort = (s.startsWith("https://") && port == 443);
        String h = host.contains(":") ? "[" + host + "]" : host;
        return u.getScheme() + "://" + h + (defaultPort ? "" : ":" + port);
    }

    /** Returns the server's /api/info, or null if it isn't a Gundil Music server. */
    static JSONObject probe(String base, int timeoutMs) {
        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL(base + "/api/info").openConnection();
            c.setConnectTimeout(timeoutMs);
            c.setReadTimeout(timeoutMs * 4);
            int code = c.getResponseCode();
            if (code == 401) {
                JSONObject o = new JSONObject();
                o.put("name", "Gundil Music");
                o.put("hostname", Uri.parse(base).getHost());
                o.put("locked", true);
                return o;
            }
            if (code != 200) return null;
            try (InputStream in = c.getInputStream()) {
                ByteArrayOutputStream buf = new ByteArrayOutputStream();
                byte[] b = new byte[4096];
                int n;
                while ((n = in.read(b)) > 0 && buf.size() < 65536) buf.write(b, 0, n);
                JSONObject o = new JSONObject(buf.toString("UTF-8"));
                return "Gundil Music".equals(o.optString("name")) ? o : null;
            }
        } catch (Exception e) {
            return null;
        } finally {
            if (c != null) c.disconnect();
        }
    }

    /** Finds Gundil Music servers on the local network (same /24 as this phone). */
    static List<JSONObject> scan(Context ctx) {
        List<JSONObject> found = Collections.synchronizedList(new ArrayList<>());
        String prefix = null;
        try {
            ConnectivityManager cm = (ConnectivityManager) ctx.getSystemService(Context.CONNECTIVITY_SERVICE);
            Network net = cm.getActiveNetwork();
            LinkProperties lp = net != null ? cm.getLinkProperties(net) : null;
            if (lp != null) {
                for (LinkAddress la : lp.getLinkAddresses()) {
                    InetAddress a = la.getAddress();
                    if (a instanceof Inet4Address && !a.isLoopbackAddress()) {
                        String ip = a.getHostAddress();
                        prefix = ip.substring(0, ip.lastIndexOf('.') + 1);
                        break;
                    }
                }
            }
        } catch (Exception ignored) { }
        ExecutorService scanPool = Executors.newFixedThreadPool(48);
        List<String> bases = new ArrayList<>();
        bases.add("http://127.0.0.1:" + PORT); // a server running on this phone (Termux)
        if (prefix != null) for (int i = 1; i < 255; i++) bases.add("http://" + prefix + i + ":" + PORT);
        for (String base : bases) {
            scanPool.execute(() -> {
                JSONObject info = probe(base, 700);
                if (info != null) {
                    try { info.put("url", base); } catch (Exception ignored) { }
                    found.add(info);
                }
            });
        }
        scanPool.shutdown();
        try { scanPool.awaitTermination(20, TimeUnit.SECONDS); } catch (InterruptedException ignored) { }
        scanPool.shutdownNow();
        return found;
    }

    private void js(String code) {
        WebHolder.js(code);
    }

    /**
     * Methods the web pages can call as window.GundilAndroid.*. It lives as
     * long as the WebView, so it uses the application, not a screen.
     */
    static final class Bridge {
        private final Context app;
        private final SharedPreferences prefs;

        Bridge(Context app) {
            this.app = app;
            this.prefs = prefs(app);
        }

        private static void js(String code) {
            WebHolder.js(code);
        }

        @JavascriptInterface
        public String server() {
            return prefs.getString("server", "");
        }

        @JavascriptInterface
        public void scan() {
            pool.execute(() -> {
                JSONArray arr = new JSONArray();
                for (JSONObject o : MainActivity.scan(app)) arr.put(o);
                js("window.onServers && window.onServers(" + arr + ")");
            });
        }

        @JavascriptInterface
        public void connect(String input) {
            String base = normalize(input);
            if (base.isEmpty()) {
                js("window.onConnectError && window.onConnectError('Enter the server address, like 192.168.1.20')");
                return;
            }
            pool.execute(() -> {
                JSONObject info = probe(base, 4000);
                if (info == null) {
                    js("window.onConnectError && window.onConnectError(" + JSONObject.quote("No Gundil Music server answered at " + base) + ")");
                    return;
                }
                prefs.edit().putString("server", base).apply();
                WebHolder.ui.post(() -> {
                    if (WebHolder.web == null) return;
                    WebHolder.web.clearHistory();
                    WebHolder.web.loadUrl(base + "/");
                });
            });
        }

        @JavascriptInterface
        public void changeServer() {
            WebHolder.ui.post(() -> showSetup(null));
        }

        @JavascriptInterface
        public String version() {
            return BuildConfigVersion.name(app);
        }

        /** The web app reports what is playing so the notification can show it. */
        @JavascriptInterface
        public void update(String json) {
            try {
                JSONObject o = new JSONObject(json);
                Intent i = new Intent(app, PlaybackService.class);
                i.setAction(PlaybackService.ACTION_UPDATE);
                i.putExtra("title", o.optString("title"));
                i.putExtra("artist", o.optString("artist"));
                i.putExtra("thumb", o.optString("thumb"));
                i.putExtra("playing", o.optBoolean("playing"));
                i.putExtra("local", o.optBoolean("local", true));
                i.putExtra("hasTrack", o.optBoolean("hasTrack"));
                i.putExtra("where", o.optString("where"));
                if (o.optBoolean("hasTrack")) {
                    PlaybackService svc = PlaybackService.instance;
                    if (svc != null) {
                        WebHolder.ui.post(() -> svc.apply(i)); // already running: no new start needed
                    } else {
                        try {
                            if (Build.VERSION.SDK_INT >= 26) app.startForegroundService(i);
                            else app.startService(i);
                        } catch (RuntimeException ignored) {
                            // Not allowed from the background; the next update while visible starts it.
                        }
                    }
                } else {
                    app.stopService(new Intent(app, PlaybackService.class));
                }
            } catch (Exception ignored) { }
        }
    }

    /**
     * A WebView that never learns it was hidden. When the app is minimised or
     * the screen turns off, Android tells WebViews they are invisible and
     * Chromium then throttles the page and may suspend its media. Gundil
     * Music must keep playing and moving to the next song, so we always
     * report "visible".
     */
    static final class KeepPlayingWebView extends WebView {
        KeepPlayingWebView(Context c) { super(c); }

        @Override
        protected void onWindowVisibilityChanged(int visibility) {
            if (visibility != android.view.View.GONE) super.onWindowVisibilityChanged(android.view.View.VISIBLE);
        }
    }

    /** Reads versionName from the package (no BuildConfig needed). */
    static final class BuildConfigVersion {
        static String name(Context c) {
            try {
                return c.getPackageManager().getPackageInfo(c.getPackageName(), 0).versionName;
            } catch (Exception e) {
                return "";
            }
        }
    }
}

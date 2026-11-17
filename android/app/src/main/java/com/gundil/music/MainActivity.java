package com.gundil.music;

import android.Manifest;
import android.annotation.SuppressLint;
import android.app.Activity;
import android.app.AlertDialog;
import android.content.Context;
import android.content.Intent;
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
import android.view.ViewGroup;
import android.webkit.HttpAuthHandler;
import android.webkit.JavascriptInterface;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceError;
import android.webkit.WebResourceRequest;
import android.webkit.WebSettings;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.EditText;
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

    static MainActivity instance;

    private WebView web;
    private SharedPreferences prefs;
    private final Handler ui = new Handler(Looper.getMainLooper());
    private final ExecutorService pool = Executors.newCachedThreadPool();
    private boolean authAsked;

    @SuppressLint("SetJavaScriptEnabled")
    @Override
    protected void onCreate(Bundle state) {
        super.onCreate(state);
        instance = this;
        prefs = getSharedPreferences("gundil", MODE_PRIVATE);

        if ((getApplicationInfo().flags & android.content.pm.ApplicationInfo.FLAG_DEBUGGABLE) != 0) {
            WebView.setWebContentsDebuggingEnabled(true); // debug builds only, for automated tests
        }
        // "adb shell am start ... --es server 192.168.1.20" sets the server directly.
        String given = getIntent() != null ? getIntent().getStringExtra("server") : null;
        if (given != null && !normalize(given).isEmpty()) prefs.edit().putString("server", normalize(given)).apply();

        web = new KeepPlayingWebView(this);
        web.setBackgroundColor(Color.parseColor("#030303"));
        web.setLayoutParams(new ViewGroup.LayoutParams(ViewGroup.LayoutParams.MATCH_PARENT, ViewGroup.LayoutParams.MATCH_PARENT));
        setContentView(web);

        WebSettings ws = web.getSettings();
        ws.setJavaScriptEnabled(true);
        ws.setDomStorageEnabled(true);
        ws.setMediaPlaybackRequiresUserGesture(false);
        ws.setMixedContentMode(WebSettings.MIXED_CONTENT_ALWAYS_ALLOW);
        ws.setUserAgentString(ws.getUserAgentString() + " GundilMusicAndroid/" + BuildConfigVersion.name(this));
        web.addJavascriptInterface(new Bridge(), "GundilAndroid");
        web.setWebChromeClient(new WebChromeClient());
        web.setWebViewClient(new WebViewClient() {
            @Override
            public boolean shouldOverrideUrlLoading(WebView view, WebResourceRequest req) {
                Uri u = req.getUrl();
                String server = prefs.getString("server", "");
                if (u.getScheme() != null && u.getScheme().startsWith("http") && !server.isEmpty() && u.toString().startsWith(server)) {
                    return false;
                }
                if ("file".equals(u.getScheme())) return false;
                // Links that leave the server (e.g. "Share" links) open in the browser.
                try { startActivity(new Intent(Intent.ACTION_VIEW, u)); } catch (Exception ignored) { }
                return true;
            }

            @Override
            public void onReceivedHttpAuthRequest(WebView view, HttpAuthHandler handler, String host, String realm) {
                String user = prefs.getString("user", ""), pass = prefs.getString("pass", "");
                if (!authAsked && !user.isEmpty()) {
                    authAsked = true;
                    handler.proceed(user, pass);
                    return;
                }
                askLogin(handler);
            }

            @Override
            public void onPageFinished(WebView view, String url) {
                if (url.startsWith("http")) authAsked = false;
            }

            @Override
            public boolean onRenderProcessGone(WebView view, android.webkit.RenderProcessGoneDetail detail) {
                // The page's renderer crashed or was killed: start over instead of letting Android close the app.
                android.util.Log.w("GundilMusic", "WebView renderer gone (crash=" + detail.didCrash() + "), reloading");
                recreate();
                return true;
            }

            @Override
            public void onReceivedError(WebView view, WebResourceRequest req, WebResourceError err) {
                if (req.isForMainFrame() && !req.getUrl().toString().startsWith("file:")) {
                    showSetup("Can't reach " + prefs.getString("server", "the server") + ". Check that it is running and that this phone is on the same network.");
                }
            }
        });

        if (Build.VERSION.SDK_INT >= 33 && checkSelfPermission(Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED) {
            requestPermissions(new String[]{Manifest.permission.POST_NOTIFICATIONS}, 1);
        }

        if (state != null) {
            web.restoreState(state);
        } else {
            String server = prefs.getString("server", "");
            if (server.isEmpty()) showSetup(null);
            else web.loadUrl(server + "/");
        }
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
        super.onStop();
    }

    @Override
    protected void onDestroy() {
        if (instance == this) instance = null;
        stopService(new Intent(this, PlaybackService.class));
        web.destroy();
        pool.shutdownNow();
        super.onDestroy();
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

    private void showSetup(String error) {
        String url = SETUP_URL + (error != null ? "#error=" + Uri.encode(error) : "");
        web.loadUrl(url);
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
    void control(String cmd) {
        ui.post(() -> web.evaluateJavascript("window.gundilControl && window.gundilControl('" + cmd + "')", null));
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
    List<JSONObject> scan() {
        List<JSONObject> found = Collections.synchronizedList(new ArrayList<>());
        String prefix = null;
        try {
            ConnectivityManager cm = (ConnectivityManager) getSystemService(Context.CONNECTIVITY_SERVICE);
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
        ui.post(() -> web.evaluateJavascript(code, null));
    }

    /** Methods the web pages can call as window.GundilAndroid.*. */
    class Bridge {
        @JavascriptInterface
        public String server() {
            return prefs.getString("server", "");
        }

        @JavascriptInterface
        public void scan() {
            pool.execute(() -> {
                JSONArray arr = new JSONArray();
                for (JSONObject o : MainActivity.this.scan()) arr.put(o);
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
                ui.post(() -> {
                    web.clearHistory();
                    web.loadUrl(base + "/");
                });
            });
        }

        @JavascriptInterface
        public void changeServer() {
            ui.post(() -> showSetup(null));
        }

        @JavascriptInterface
        public String version() {
            return BuildConfigVersion.name(MainActivity.this);
        }

        /** The web app reports what is playing so the notification can show it. */
        @JavascriptInterface
        public void update(String json) {
            try {
                JSONObject o = new JSONObject(json);
                Intent i = new Intent(MainActivity.this, PlaybackService.class);
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
                        ui.post(() -> svc.apply(i)); // already running: no new start needed
                    } else {
                        try {
                            if (Build.VERSION.SDK_INT >= 26) startForegroundService(i);
                            else startService(i);
                        } catch (RuntimeException ignored) {
                            // Not allowed from the background; the next update while visible starts it.
                        }
                    }
                } else {
                    stopService(new Intent(MainActivity.this, PlaybackService.class));
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

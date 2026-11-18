package com.gundil.music;

import android.content.Context;
import android.net.Uri;
import android.util.Base64;
import android.webkit.CookieManager;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;

import java.io.ByteArrayInputStream;
import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.Arrays;
import java.util.HashMap;
import java.util.HashSet;
import java.util.Map;
import java.util.Set;

/**
 * Keeps a copy of the web app's own files (the page, its script and styles)
 * on the phone, so the app still opens when the server can't be reached and
 * can play the songs downloaded on this phone. In a browser a service worker
 * does this, but browsers only allow those on https, and Gundil Music
 * servers on a home network are plain http.
 */
final class Shell {
    private static final Set<String> FILES = new HashSet<>(Arrays.asList(
            "/", "/index.html", "/app.js", "/style.css", "/icon.svg", "/icon-192.png", "/manifest.webmanifest"));

    private Shell() { }

    /** A response for one of the app's own files, or null to let the WebView load it normally. */
    static WebResourceResponse serve(Context app, String server, WebResourceRequest req) {
        if (!"GET".equals(req.getMethod()) || server.isEmpty()) return null;
        Uri u = req.getUrl();
        String url = u.toString();
        if (!url.startsWith(server + "/")) return null;
        String path = u.getPath() == null || u.getPath().isEmpty() ? "/" : u.getPath();
        if (!FILES.contains(path)) return null;
        String name = path.equals("/") ? "index.html" : path.substring(1);
        File dir = new File(app.getFilesDir(), "shell");
        File saved = new File(dir, name);

        HttpURLConnection c = null;
        try {
            c = (HttpURLConnection) new URL(url).openConnection();
            c.setInstanceFollowRedirects(false);
            c.setConnectTimeout(4000);
            c.setReadTimeout(20000);
            c.setUseCaches(false);
            for (Map.Entry<String, String> h : req.getRequestHeaders().entrySet()) {
                String k = h.getKey();
                if (k.equalsIgnoreCase("If-None-Match") || k.equalsIgnoreCase("If-Modified-Since") || k.equalsIgnoreCase("Range")) continue;
                c.setRequestProperty(k, h.getValue());
            }
            String cookie = CookieManager.getInstance().getCookie(url);
            if (cookie != null && !cookie.isEmpty()) c.setRequestProperty("Cookie", cookie);
            String user = MainActivity.prefs(app).getString("user", ""), pass = MainActivity.prefs(app).getString("pass", "");
            if (!user.isEmpty()) {
                c.setRequestProperty("Authorization", "Basic " + Base64.encodeToString((user + ":" + pass).getBytes("UTF-8"), Base64.NO_WRAP));
            }
            int code = c.getResponseCode();
            if (code != 200) return null; // sign-in prompts, redirects and errors: the WebView handles them
            byte[] body;
            try (InputStream in = c.getInputStream()) { body = readAll(in); }
            save(dir, saved, body);
            String type = c.getContentType();
            return response(type != null ? type : typeOf(name), body, false);
        } catch (IOException e) {
            // The server is off or unreachable: use the copy from last time, if there is one.
            if (!saved.isFile()) return null;
            try (InputStream in = new FileInputStream(saved)) {
                return response(typeOf(name), readAll(in), true);
            } catch (IOException e2) {
                return null;
            }
        } finally {
            if (c != null) c.disconnect();
        }
    }

    private static WebResourceResponse response(String contentType, byte[] body, boolean offline) {
        String mime = contentType, charset = null;
        int semi = contentType.indexOf(';');
        if (semi >= 0) {
            mime = contentType.substring(0, semi).trim();
            int cs = contentType.toLowerCase().indexOf("charset=");
            if (cs >= 0) charset = contentType.substring(cs + 8).trim();
        }
        if (charset == null && (mime.startsWith("text/") || mime.contains("javascript") || mime.contains("json"))) charset = "utf-8";
        Map<String, String> headers = new HashMap<>();
        headers.put("Cache-Control", "no-cache");
        if (offline) headers.put("X-Gundil-Offline", "1");
        return new WebResourceResponse(mime, charset, 200, "OK", headers, new ByteArrayInputStream(body));
    }

    private static void save(File dir, File file, byte[] body) {
        try {
            if (!dir.isDirectory() && !dir.mkdirs()) return;
            File tmp = new File(dir, file.getName() + ".tmp");
            try (FileOutputStream out = new FileOutputStream(tmp)) { out.write(body); }
            if (!tmp.renameTo(file)) tmp.delete();
        } catch (IOException ignored) { }
    }

    private static byte[] readAll(InputStream in) throws IOException {
        ByteArrayOutputStream buf = new ByteArrayOutputStream();
        byte[] b = new byte[16384];
        int n;
        while ((n = in.read(b)) > 0) buf.write(b, 0, n);
        return buf.toByteArray();
    }

    private static String typeOf(String name) {
        if (name.endsWith(".html")) return "text/html; charset=utf-8";
        if (name.endsWith(".js")) return "text/javascript; charset=utf-8";
        if (name.endsWith(".css")) return "text/css; charset=utf-8";
        if (name.endsWith(".svg")) return "image/svg+xml";
        if (name.endsWith(".png")) return "image/png";
        if (name.endsWith(".webmanifest")) return "application/manifest+json";
        return "application/octet-stream";
    }
}

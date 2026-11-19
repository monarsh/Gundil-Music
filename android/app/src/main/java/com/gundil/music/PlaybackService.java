package com.gundil.music;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.PendingIntent;
import android.app.Service;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.media.AudioManager;
import android.content.pm.ServiceInfo;
import android.graphics.Bitmap;
import android.graphics.BitmapFactory;
import android.media.MediaMetadata;
import android.media.session.MediaSession;
import android.media.session.PlaybackState;
import android.net.wifi.WifiManager;
import android.os.Build;
import android.os.Handler;
import android.os.IBinder;
import android.os.Looper;
import android.os.PowerManager;
import android.os.SystemClock;

import java.io.InputStream;
import java.net.HttpURLConnection;
import java.net.URL;

/**
 * Keeps Gundil Music alive while music plays with the screen off, and shows
 * the media notification. Notification, lock-screen and Bluetooth headset
 * buttons are passed to the web app.
 */
public class PlaybackService extends Service {
    static final String ACTION_UPDATE = "com.gundil.music.UPDATE";
    static final String ACTION_CONTROL = "com.gundil.music.CONTROL";
    private static final String CHANNEL = "playback";
    private static final int ID = 1;

    static volatile PlaybackService instance;

    private MediaSession session;
    private PowerManager.WakeLock wake;
    private WifiManager.WifiLock wifi;
    private String title = "", artist = "", thumb = "", where = "", artFor = "";
    private boolean playing, local = true;
    /** Where the song is (ms) when the page last said, and when; how long it is. */
    private long position, positionAt, duration;
    /** What the notification and lock screen show now, and whether Android let us stay in the foreground. */
    private volatile String shown = "";
    private volatile boolean shownPlaying, foreground;
    private volatile String lastError = "";
    private Bitmap art;

    /**
     * Another app took over the sound (or a call came) and the music paused
     * by itself. Gundil Music stays ready, and plays on once nothing else is
     * playing, as long as that's within RESUME_FOR.
     */
    private boolean interrupted;
    private long interruptedAt;
    private int quietChecks;
    private static final long RESUME_FOR = 30 * 60 * 1000L;
    /** Paused by you: stay in the foreground (not closed by Android) this long. */
    private static final long KEEP_PAUSED = 15 * 60 * 1000L;
    private final Handler handler = new Handler(Looper.getMainLooper());

    private final Runnable watchForQuiet = new Runnable() {
        @Override
        public void run() {
            if (!interrupted || playing || !local) return;
            if (SystemClock.elapsedRealtime() - interruptedAt > RESUME_FOR) {
                interrupted = false; // too long ago: leave it paused
                refresh();
                return;
            }
            AudioManager am = (AudioManager) getSystemService(Context.AUDIO_SERVICE);
            boolean busy = am.isMusicActive() || am.getMode() != AudioManager.MODE_NORMAL;
            quietChecks = busy ? 0 : quietChecks + 1;
            if (quietChecks >= 2) {
                // The other app has stopped (and no call is on): play on.
                quietChecks = 0;
                control("play");
                handler.postDelayed(this, 5000);
            } else {
                handler.postDelayed(this, 2000);
            }
        }
    };

    /** Paused for a while: let the notification go, and Android close the app if it needs to. */
    private final Runnable letGo = () -> {
        if (!playing && !interrupted && Build.VERSION.SDK_INT >= 24) stopForeground(STOP_FOREGROUND_DETACH);
    };

    /** Headphones unplugged or a Bluetooth speaker disconnected: pause, don't blast the phone speaker. */
    private final BroadcastReceiver noisy = new BroadcastReceiver() {
        @Override
        public void onReceive(Context c, Intent i) {
            if (AudioManager.ACTION_AUDIO_BECOMING_NOISY.equals(i.getAction()) && playing && local) control("pause");
        }
    };

    @Override
    public void onCreate() {
        super.onCreate();
        instance = this;
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel ch = new NotificationChannel(CHANNEL, "Playback", NotificationManager.IMPORTANCE_LOW);
            ch.setShowBadge(false);
            getSystemService(NotificationManager.class).createNotificationChannel(ch);
        }
        session = new MediaSession(this, "GundilMusic");
        session.setCallback(new MediaSession.Callback() {
            @Override public void onPlay() { control("play"); }
            @Override public void onPause() { control("pause"); }
            @Override public void onSkipToNext() { control("next"); }
            @Override public void onSkipToPrevious() { control("prev"); }
            @Override public void onStop() { control("pause"); }
            @Override public void onSeekTo(long ms) { control("seek:" + (ms / 1000.0)); }
        });
        session.setActive(true);

        IntentFilter f = new IntentFilter(AudioManager.ACTION_AUDIO_BECOMING_NOISY);
        if (Build.VERSION.SDK_INT >= 33) registerReceiver(noisy, f, Context.RECEIVER_NOT_EXPORTED);
        else registerReceiver(noisy, f);

        PowerManager pm = (PowerManager) getSystemService(Context.POWER_SERVICE);
        wake = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "GundilMusic:playback");
        wake.setReferenceCounted(false);
        WifiManager wm = (WifiManager) getApplicationContext().getSystemService(Context.WIFI_SERVICE);
        if (wm != null) {
            int mode = Build.VERSION.SDK_INT >= 29 ? WifiManager.WIFI_MODE_FULL_LOW_LATENCY : WifiManager.WIFI_MODE_FULL_HIGH_PERF;
            wifi = wm.createWifiLock(mode, "GundilMusic:stream");
            wifi.setReferenceCounted(false);
        }
    }

    private void control(String cmd) {
        MainActivity.control(cmd); // works with or without the app's screen open
    }

    /** Whether this phone itself is playing music right now (or will again, once another app stops). */
    static boolean playingHere() {
        PlaybackService s = instance;
        return s != null && s.local && (s.playing || s.interrupted);
    }

    /** The app was swiped away from Recents. Keep going only while music plays here. */
    @Override
    public void onTaskRemoved(Intent rootIntent) {
        if (!playingHere()) {
            WebHolder.ui.post(WebHolder::destroy);
            stopSelf();
        }
        super.onTaskRemoved(rootIntent);
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        if (intent != null && ACTION_CONTROL.equals(intent.getAction())) {
            control(intent.getStringExtra("cmd"));
        } else if (intent != null && ACTION_UPDATE.equals(intent.getAction())) {
            apply(intent);
            return START_NOT_STICKY;
        }
        refresh();
        return START_NOT_STICKY;
    }

    /** Applies a "now playing" update from the web app. */
    void apply(Intent intent) {
        {
            title = intent.getStringExtra("title");
            artist = intent.getStringExtra("artist");
            String t = intent.getStringExtra("thumb");
            playing = intent.getBooleanExtra("playing", false);
            local = intent.getBooleanExtra("local", true);
            boolean wasInterrupted = interrupted;
            interrupted = !playing && local && intent.getBooleanExtra("interrupted", false);
            if (interrupted && !wasInterrupted) {
                interruptedAt = SystemClock.elapsedRealtime();
                quietChecks = 0;
                handler.removeCallbacks(watchForQuiet);
                handler.postDelayed(watchForQuiet, 2000);
            }
            where = intent.getStringExtra("where");
            position = intent.getLongExtra("position", 0);
            positionAt = SystemClock.elapsedRealtime();
            duration = intent.getLongExtra("duration", 0);
            if (t != null && !t.equals(artFor)) {
                artFor = t;
                art = null;
                loadArt(t);
            }
            thumb = t;
        }
        scheduleRefresh();
    }

    private final Runnable refreshNow = this::refresh;

    /**
     * Shows the latest update a moment later. A song change sends several
     * updates in quick succession, and Android drops notification updates
     * that come faster than a few a second, so the notification could be
     * left showing an earlier song. Waiting for them to settle shows the
     * last one.
     */
    private void scheduleRefresh() {
        handler.removeCallbacks(refreshNow);
        handler.postDelayed(refreshNow, 300);
    }

    private void refresh() {
        handler.removeCallbacks(refreshNow);
        // Hold locks only while this phone itself is streaming audio (or waits to play on).
        boolean active = local && (playing || interrupted);
        if (active) {
            if (!wake.isHeld()) wake.acquire(6 * 60 * 60 * 1000L);
            if (wifi != null && !wifi.isHeld()) wifi.acquire();
        } else {
            if (wake.isHeld()) wake.release();
            if (wifi != null && wifi.isHeld()) wifi.release();
        }

        try {
            MediaMetadata.Builder md = new MediaMetadata.Builder()
                    .putString(MediaMetadata.METADATA_KEY_TITLE, title)
                    .putString(MediaMetadata.METADATA_KEY_ARTIST, artist);
            if (duration > 0) md.putLong(MediaMetadata.METADATA_KEY_DURATION, duration);
            if (art != null) md.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, art);
            session.setActive(true);
            session.setMetadata(md.build());
            // The position lets the notification and lock screen show the
            // song's progress bar (and drag it), as in YouTube Music.
            session.setPlaybackState(new PlaybackState.Builder()
                    .setActions(PlaybackState.ACTION_PLAY | PlaybackState.ACTION_PAUSE | PlaybackState.ACTION_PLAY_PAUSE
                            | PlaybackState.ACTION_SKIP_TO_NEXT | PlaybackState.ACTION_SKIP_TO_PREVIOUS | PlaybackState.ACTION_SEEK_TO)
                    .setState(playing ? PlaybackState.STATE_PLAYING : PlaybackState.STATE_PAUSED,
                            duration > 0 ? position : PlaybackState.PLAYBACK_POSITION_UNKNOWN, playing ? 1f : 0f, positionAt)
                    .build());
            shown = title == null ? "" : title;
            shownPlaying = playing;
        } catch (RuntimeException e) {
            lastError = "session: " + e;
        }

        Notification.Builder b = Build.VERSION.SDK_INT >= 26 ? new Notification.Builder(this, CHANNEL) : new Notification.Builder(this);
        Intent open = new Intent(this, MainActivity.class).setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        b.setSmallIcon(R.drawable.ic_notification)
                .setContentTitle(title)
                .setContentText(artist)
                .setSubText(local ? null : where)
                .setLargeIcon(art)
                .setOngoing(playing || interrupted)
                .setShowWhen(false)
                .setVisibility(Notification.VISIBILITY_PUBLIC)
                .setContentIntent(PendingIntent.getActivity(this, 0, open, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT))
                .addAction(action(android.R.drawable.ic_media_previous, "Previous", "prev", 1))
                .addAction(playing ? action(android.R.drawable.ic_media_pause, "Pause", "pause", 2) : action(android.R.drawable.ic_media_play, "Play", "play", 2))
                .addAction(action(android.R.drawable.ic_media_next, "Next", "next", 3))
                .setStyle(new Notification.MediaStyle().setMediaSession(session.getSessionToken()).setShowActionsInCompactView(0, 1, 2));
        Notification n = b.build();
        try {
            if (Build.VERSION.SDK_INT >= 29) startForeground(ID, n, ServiceInfo.FOREGROUND_SERVICE_TYPE_MEDIA_PLAYBACK);
            else startForeground(ID, n);
            foreground = true;
        } catch (RuntimeException e) {
            // Android may refuse a foreground start from the background; still show the controls.
            foreground = false;
            lastError = "foreground: " + e;
            try {
                getSystemService(NotificationManager.class).notify(ID, n);
            } catch (RuntimeException e2) {
                lastError = "notify: " + e2;
            }
        }
        handler.removeCallbacks(letGo);
        if (!playing && !interrupted) {
            // Paused: after a while the notification may be swiped away, and
            // Android may close the app. Straight away, Android closed it
            // within minutes with the screen off.
            handler.postDelayed(letGo, local ? KEEP_PAUSED : 0);
        }
    }

    private Notification.Action action(int icon, String label, String cmd, int req) {
        Intent i = new Intent(this, PlaybackService.class).setAction(ACTION_CONTROL).putExtra("cmd", cmd);
        PendingIntent pi = PendingIntent.getService(this, req, i, PendingIntent.FLAG_IMMUTABLE | PendingIntent.FLAG_UPDATE_CURRENT);
        return new Notification.Action.Builder(icon, label, pi).build();
    }

    private void loadArt(String url) {
        if (url == null || url.isEmpty()) return;
        new Thread(() -> {
            HttpURLConnection c = null;
            try {
                c = (HttpURLConnection) new URL(url).openConnection();
                c.setConnectTimeout(8000);
                c.setReadTimeout(8000);
                try (InputStream in = c.getInputStream()) {
                    Bitmap bmp = BitmapFactory.decodeStream(in);
                    if (bmp != null && url.equals(artFor)) {
                        art = bmp;
                        handler.post(this::scheduleRefresh);
                    }
                }
            } catch (Exception ignored) {
            } finally {
                if (c != null) c.disconnect();
            }
        }).start();
    }

    @Override
    public void onDestroy() {
        if (instance == this) instance = null;
        handler.removeCallbacksAndMessages(null);
        try { unregisterReceiver(noisy); } catch (RuntimeException ignored) { }
        if (wake.isHeld()) wake.release();
        if (wifi != null && wifi.isHeld()) wifi.release();
        session.release();
        super.onDestroy();
    }

    /** What the notification shows, for the page to check against what plays (see Bridge.playbackInfo). */
    static String describe() {
        PlaybackService s = instance;
        org.json.JSONObject o = new org.json.JSONObject();
        try {
            o.put("running", s != null);
            if (s != null) {
                o.put("title", s.shown);
                o.put("playing", s.shownPlaying);
                o.put("foreground", s.foreground);
                o.put("error", s.lastError);
            }
        } catch (org.json.JSONException ignored) { }
        return o.toString();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}

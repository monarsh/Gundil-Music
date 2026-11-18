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
import android.os.IBinder;
import android.os.PowerManager;

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

    static PlaybackService instance;

    private MediaSession session;
    private PowerManager.WakeLock wake;
    private WifiManager.WifiLock wifi;
    private String title = "", artist = "", thumb = "", where = "", artFor = "";
    private boolean playing, local = true;
    private Bitmap art;

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

    /** Whether this phone itself is playing music right now. */
    static boolean playingHere() {
        PlaybackService s = instance;
        return s != null && s.playing && s.local;
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
            where = intent.getStringExtra("where");
            if (t != null && !t.equals(artFor)) {
                artFor = t;
                art = null;
                loadArt(t);
            }
            thumb = t;
        }
        refresh();
    }

    private void refresh() {
        // Hold locks only while this phone itself is streaming audio.
        if (playing && local) {
            if (!wake.isHeld()) wake.acquire(6 * 60 * 60 * 1000L);
            if (wifi != null && !wifi.isHeld()) wifi.acquire();
        } else {
            if (wake.isHeld()) wake.release();
            if (wifi != null && wifi.isHeld()) wifi.release();
        }

        MediaMetadata.Builder md = new MediaMetadata.Builder()
                .putString(MediaMetadata.METADATA_KEY_TITLE, title)
                .putString(MediaMetadata.METADATA_KEY_ARTIST, artist);
        if (art != null) md.putBitmap(MediaMetadata.METADATA_KEY_ALBUM_ART, art);
        session.setMetadata(md.build());
        session.setPlaybackState(new PlaybackState.Builder()
                .setActions(PlaybackState.ACTION_PLAY | PlaybackState.ACTION_PAUSE | PlaybackState.ACTION_PLAY_PAUSE
                        | PlaybackState.ACTION_SKIP_TO_NEXT | PlaybackState.ACTION_SKIP_TO_PREVIOUS)
                .setState(playing ? PlaybackState.STATE_PLAYING : PlaybackState.STATE_PAUSED, PlaybackState.PLAYBACK_POSITION_UNKNOWN, 1f)
                .build());

        Notification.Builder b = Build.VERSION.SDK_INT >= 26 ? new Notification.Builder(this, CHANNEL) : new Notification.Builder(this);
        Intent open = new Intent(this, MainActivity.class).setFlags(Intent.FLAG_ACTIVITY_SINGLE_TOP);
        b.setSmallIcon(R.drawable.ic_notification)
                .setContentTitle(title)
                .setContentText(artist)
                .setSubText(local ? null : where)
                .setLargeIcon(art)
                .setOngoing(playing)
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
        } catch (RuntimeException e) {
            // Android may refuse a foreground start from the background; still show the controls.
            getSystemService(NotificationManager.class).notify(ID, n);
        }
        if (!playing && Build.VERSION.SDK_INT >= 24) {
            // Paused: the notification may be swiped away, and Android may stop us.
            stopForeground(STOP_FOREGROUND_DETACH);
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
                        new android.os.Handler(getMainLooper()).post(this::refresh);
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
        try { unregisterReceiver(noisy); } catch (RuntimeException ignored) { }
        if (wake.isHeld()) wake.release();
        if (wifi != null && wifi.isHeld()) wifi.release();
        session.release();
        super.onDestroy();
    }

    @Override
    public IBinder onBind(Intent intent) {
        return null;
    }
}

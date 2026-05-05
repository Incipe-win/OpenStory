"use client";

import { useRef, useState, useEffect, useCallback } from "react";
import { cn } from "@/lib/utils/cn";
import { formatDuration } from "@/lib/utils/format";
import { Play, Pause, Volume2, VolumeX, Maximize, SkipForward, SkipBack } from "lucide-react";

interface VideoPlayerProps {
  src: string;
  poster?: string;
  className?: string;
}

export function VideoPlayer({ src, poster, className }: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(false);
  const [currentTime, setCurrentTime] = useState(0);
  const [duration, setDuration] = useState(0);
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onTimeUpdate = () => {
      setCurrentTime(video.currentTime);
      if (video.duration) {
        setProgress((video.currentTime / video.duration) * 100);
      }
    };
    const onLoaded = () => setDuration(video.duration);
    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);

    video.addEventListener("timeupdate", onTimeUpdate);
    video.addEventListener("loadedmetadata", onLoaded);
    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);

    return () => {
      video.removeEventListener("timeupdate", onTimeUpdate);
      video.removeEventListener("loadedmetadata", onLoaded);
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
    };
  }, []);

  const togglePlay = useCallback(() => {
    if (!videoRef.current) return;
    if (playing) videoRef.current.pause();
    else videoRef.current.play();
  }, [playing]);

  const toggleMute = useCallback(() => {
    if (!videoRef.current) return;
    videoRef.current.muted = !muted;
    setMuted(!muted);
  }, [muted]);

  const seek = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (!videoRef.current) return;
    const rect = e.currentTarget.getBoundingClientRect();
    const x = e.clientX - rect.left;
    const pct = x / rect.width;
    videoRef.current.currentTime = pct * videoRef.current.duration;
  }, []);

  const fullscreen = useCallback(() => {
    videoRef.current?.requestFullscreen();
  }, []);

  return (
    <div className={cn("relative group bg-black overflow-hidden cyber-chamfer", className)}>
      <video
        ref={videoRef}
        src={src}
        poster={poster}
        className="w-full h-full cursor-pointer"
        onClick={togglePlay}
        playsInline
      />

      {/* Center play button */}
      {!playing && (
        <div
          className="absolute inset-0 flex items-center justify-center bg-black/30 cursor-pointer"
          onClick={togglePlay}
        >
          <div className="w-16 h-16 bg-accent/80 cyber-chamfer-sm flex items-center justify-center hover:bg-accent transition-colors">
            <Play className="h-8 w-8 text-background ml-1" />
          </div>
        </div>
      )}

      {/* Controls */}
      <div className="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/80 to-transparent p-4 opacity-0 group-hover:opacity-100 transition-opacity">
        {/* Progress bar */}
        <div
          className="w-full h-1 bg-muted/50 mb-3 cursor-pointer rounded"
          onClick={seek}
        >
          <div
            className="h-full bg-accent transition-all"
            style={{ width: `${progress}%` }}
          />
        </div>

        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button onClick={togglePlay} className="text-white hover:text-accent transition-colors cursor-pointer">
              {playing ? <Pause className="h-5 w-5" /> : <Play className="h-5 w-5" />}
            </button>
            <button onClick={toggleMute} className="text-white hover:text-accent transition-colors cursor-pointer">
              {muted ? <VolumeX className="h-5 w-5" /> : <Volume2 className="h-5 w-5" />}
            </button>
            <span className="text-xs font-mono text-white/70">
              {formatDuration(currentTime * 1000)} / {formatDuration(duration * 1000)}
            </span>
          </div>
          <button onClick={fullscreen} className="text-white hover:text-accent transition-colors cursor-pointer">
            <Maximize className="h-5 w-5" />
          </button>
        </div>
      </div>
    </div>
  );
}

<script lang="ts">
  import { onDestroy, onMount } from "svelte";
  import { parseCameraScanCode } from "../lib/scanner.ts";

  let { enabled, busy, onCode }: { enabled: boolean; busy: boolean; onCode: (code: string) => void } = $props();
  let video: HTMLVideoElement | undefined;
  let stream: MediaStream | undefined;
  let generation = 0;
  let frameTimer: number | undefined;
  let starting = $state(false);
  let active = $state(false);
  let status = $state("Kamera belum aktif. Masukkan kode secara manual bila kamera tidak tersedia.");

  function stopCamera(message?: string) {
    generation += 1;
    starting = false;
    active = false;
    if (frameTimer !== undefined) window.clearTimeout(frameTimer);
    frameTimer = undefined;
    for (const track of stream?.getTracks() ?? []) track.stop();
    stream = undefined;
    if (video) video.srcObject = null;
    if (message) status = message;
  }

  function cameraError(cause: unknown) {
    if (cause instanceof DOMException) {
      if (cause.name === "NotAllowedError" || cause.name === "PermissionDeniedError") return "Izin kamera ditolak. Izinkan kamera pada pengaturan browser atau masukkan kode secara manual.";
      if (cause.name === "NotFoundError" || cause.name === "DevicesNotFoundError") return "Kamera tidak ditemukan. Masukkan kode tiket secara manual.";
      if (cause.name === "NotReadableError" || cause.name === "TrackStartError") return "Kamera sedang digunakan atau tidak dapat dibuka. Tutup aplikasi lain atau masukkan kode secara manual.";
      if (cause.name === "SecurityError") return "Akses kamera membutuhkan HTTPS atau localhost.";
    }
    return "Kamera tidak dapat dimulai. Periksa izin kamera atau masukkan kode secara manual.";
  }

  async function startCamera() {
    if (!enabled || busy || starting || active) return;
    stopCamera();
    const current = generation;
    starting = true;
    status = "Meminta akses kamera…";
    if (!window.isSecureContext) {
      stopCamera("Akses kamera membutuhkan HTTPS atau localhost. Masukkan kode secara manual.");
      return;
    }
    if (!navigator.mediaDevices?.getUserMedia) {
      stopCamera("Browser atau perangkat ini tidak mendukung kamera. Masukkan kode secara manual.");
      return;
    }

    try {
      const nextStream = await navigator.mediaDevices.getUserMedia({ audio: false, video: { facingMode: { ideal: "environment" } } });
      if (current !== generation) { for (const track of nextStream.getTracks()) track.stop(); return; }
      stream = nextStream;
      const { default: jsQR } = await import("jsqr");
      if (current !== generation || !video) { stopCamera(); return; }
      video.srcObject = nextStream;
      await video.play();
      if (current !== generation) { stopCamera(); return; }
      starting = false;
      active = true;
      status = "Arahkan kamera ke QR e-ticket.";
      const canvas = document.createElement("canvas");
      const context = canvas.getContext("2d", { willReadFrequently: true });
      const scanFrame = () => {
        if (current !== generation || !active || !video || !context) return;
        if (video.readyState >= HTMLMediaElement.HAVE_CURRENT_DATA && video.videoWidth > 0) {
          const scale = Math.min(1, 640 / Math.max(video.videoWidth, video.videoHeight));
          canvas.width = Math.round(video.videoWidth * scale);
          canvas.height = Math.round(video.videoHeight * scale);
          context.drawImage(video, 0, 0, canvas.width, canvas.height);
          const image = context.getImageData(0, 0, canvas.width, canvas.height);
          const result = jsQR(image.data, image.width, image.height, { inversionAttempts: "dontInvert" });
          if (result) {
            const code = parseCameraScanCode(result.data);
            if (code) {
              stopCamera("QR terbaca. Memverifikasi tiket…");
              onCode(code);
              return;
            }
            status = "QR terbaca bukan kode e-ticket. Arahkan kamera ke QR yang benar atau masukkan kode secara manual.";
          }
        }
        frameTimer = window.setTimeout(scanFrame, 200);
      };
      scanFrame();
    } catch (cause) {
      if (current === generation) stopCamera(cameraError(cause));
    }
  }

  $effect(() => { if (!enabled || busy) stopCamera(); });

  function stopForPageExit() { stopCamera("Kamera dihentikan. Tekan tombol untuk memulai pemindaian lagi."); }

  onMount(() => {
    const onVisibilityChange = () => { if (document.visibilityState === "hidden") stopForPageExit(); };
    document.addEventListener("visibilitychange", onVisibilityChange);
    window.addEventListener("pagehide", stopForPageExit);
    return () => {
      document.removeEventListener("visibilitychange", onVisibilityChange);
      window.removeEventListener("pagehide", stopForPageExit);
    };
  });

  onDestroy(() => stopCamera());
</script>

<section class="camera-scanner" aria-labelledby="camera-title">
  <div class="camera-scanner-heading"><h3 id="camera-title">Scan dengan kamera</h3><span>QR diproses di perangkat ini</span></div>
  <video bind:this={video} muted playsinline aria-label="Pratinjau kamera untuk memindai QR e-ticket" class:camera-active={active}></video>
  <div class="camera-scanner-controls">
    {#if active || starting}<button class="staff-text-button" type="button" onclick={() => stopCamera("Kamera dihentikan.")}>Hentikan kamera</button>
    {:else}<button class="scan-submit camera-start" type="button" disabled={!enabled || busy} onclick={startCamera}>{busy ? "Memverifikasi…" : "Aktifkan kamera"}</button>{/if}
    <p role="status" aria-live="polite">{status}</p>
  </div>
</section>

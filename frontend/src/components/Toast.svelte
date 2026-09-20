<script lang="ts">
  let { message, tone = "info", id, onDismiss }: { message: string; tone?: "success" | "info" | "error"; id: number; onDismiss: () => void } = $props();
  let paused = $state(false);
  $effect(() => {
    if (paused) return;
    const timer = window.setTimeout(() => { if (!document.activeElement?.closest(".toast")) onDismiss(); }, 5_000);
    return () => window.clearTimeout(timer);
  });
</script>

{#key id}<div onmouseenter={() => paused = true} onmouseleave={() => paused = false} onfocusin={() => paused = true} onfocusout={() => paused = false} class:toast-success={tone === "success"} class:toast-error={tone === "error"} class="toast" role={tone === "error" ? "alert" : "status"} aria-live={tone === "error" ? "assertive" : "polite"}>
  <span class="toast-mark" aria-hidden="true">{tone === "success" ? "✓" : tone === "error" ? "!" : "i"}</span>
  <span>{message}</span>
  <button type="button" aria-label="Tutup notifikasi" onclick={onDismiss}>×</button>
</div>{/key}

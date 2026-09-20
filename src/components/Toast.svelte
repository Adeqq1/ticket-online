<script lang="ts">
  let { message, tone = "info", onDismiss }: { message: string; tone?: "success" | "info" | "error"; onDismiss: () => void } = $props();
  $effect(() => {
    const timer = window.setTimeout(() => onDismiss(), 5_000);
    return () => window.clearTimeout(timer);
  });
</script>

<div class:toast-success={tone === "success"} class:toast-error={tone === "error"} class="toast" role={tone === "error" ? "alert" : "status"} aria-live={tone === "error" ? "assertive" : "polite"}>
  <span class="toast-mark" aria-hidden="true">{tone === "success" ? "✓" : tone === "error" ? "!" : "i"}</span>
  <span>{message}</span>
  <button type="button" aria-label="Tutup notifikasi" onclick={onDismiss}>×</button>
</div>

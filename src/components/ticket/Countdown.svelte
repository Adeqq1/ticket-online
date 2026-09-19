<script lang="ts">
  import { onMount } from "svelte";
  import { timeRemaining } from "../../lib/tickets.ts";
  let { startsAt }: { startsAt: string } = $props();
  let remaining = $state(timeRemaining(""));
  let announced = $state("Sisa waktu event dapat dibaca pada penghitung waktu.");
  const text = $derived(remaining.started ? "Event telah dimulai" : `${remaining.days} hari · ${String(remaining.hours).padStart(2, "0")} jam · ${String(remaining.minutes).padStart(2, "0")} menit · ${String(remaining.seconds).padStart(2, "0")} detik`);
  onMount(() => {
    remaining = timeRemaining(startsAt);
    if (remaining.started) { announced = "Event telah dimulai"; return; }
    const timer = window.setInterval(() => { remaining = timeRemaining(startsAt); if (remaining.started) { announced = "Event telah dimulai"; window.clearInterval(timer); } }, 1_000);
    return () => window.clearInterval(timer);
  });
</script>

<div class="countdown"><span class="pass-label">Event dimulai dalam</span><strong aria-live="off">{text}</strong><span class="sr-only" aria-live="polite">{announced}</span></div>

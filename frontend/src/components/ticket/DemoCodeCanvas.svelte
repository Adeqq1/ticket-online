<script lang="ts">
  import { onMount } from "svelte";
  let { reference }: { reference: string } = $props();
  let canvas: HTMLCanvasElement;
  onMount(() => {
    const context = canvas.getContext("2d");
    if (!context) return;
    context.fillStyle = "#fff"; context.fillRect(0, 0, canvas.width, canvas.height);
    const cells = 25; const size = canvas.width / cells; const seed = [...reference].reduce((total, character) => total * 31 + character.charCodeAt(0), 7) >>> 0;
    const isFinder = (x: number, y: number) => (x < 7 && y < 7) || (x >= cells - 7 && y < 7) || (x < 7 && y >= cells - 7);
    for (let y = 0; y < cells; y++) for (let x = 0; x < cells; x++) {
      let on = ((seed + x * 17 + y * 31 + x * y) % 5) < 2;
      if (isFinder(x, y)) { const left = x < 7 ? 0 : cells - 7; const top = y < 7 ? 0 : cells - 7; const dx = x - left; const dy = y - top; on = dx === 0 || dx === 6 || dy === 0 || dy === 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4); }
      if (on) { context.fillStyle = "#17211f"; context.fillRect(x * size, y * size, Math.ceil(size), Math.ceil(size)); }
    }
  });
</script>

<canvas bind:this={canvas} class="qr-code" width="180" height="180" aria-hidden="true"></canvas>

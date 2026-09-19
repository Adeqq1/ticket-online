<script lang="ts">
  type Item = { question: string; answer: string };

  let { items, name }: { items: Item[]; name: string } = $props();
  let openIndex = $state<number | null>(null);

  function toggle(index: number) {
    openIndex = openIndex === index ? null : index;
  }
</script>

<div class="accordion" aria-label={name}>
  {#each items as item, index}
    {@const panelId = `${name.toLowerCase().replaceAll(" ", "-")}-${index}`}
    <div class="accordion-item">
      <button class="accordion-trigger" type="button" aria-expanded={openIndex === index} aria-controls={panelId} onclick={() => toggle(index)}>
        <span>{item.question}</span>
        <span class="accordion-icon" aria-hidden="true">+</span>
      </button>
      {#if openIndex === index}
        <div class="accordion-panel" id={panelId} role="region">
          <p>{item.answer}</p>
        </div>
      {/if}
    </div>
  {/each}
</div>

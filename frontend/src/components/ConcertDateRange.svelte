<script lang="ts">
  import { dateRangeError } from "../lib/filters.ts";
  let { dateFrom = $bindable(""), dateTo = $bindable("") }: { dateFrom?: string; dateTo?: string } = $props();
  const id = $props.id();
  const error = $derived(dateRangeError(dateFrom, dateTo));
</script>

<fieldset>
  <legend class="sr-only">Tanggal konser (WIB)</legend>
  <div class="date-inputs">
    <label for="{id}-from">Tanggal mulai (WIB)<input id="{id}-from" name="dateFrom" type="date" bind:value={dateFrom} aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} /></label>
    <label for="{id}-to">Tanggal akhir (WIB)<input id="{id}-to" name="dateTo" type="date" bind:value={dateTo} aria-invalid={Boolean(error)} aria-describedby={error ? `${id}-error` : undefined} /></label>
  </div>
  <p id="{id}-error" class="field-error" aria-live="polite">{error}</p>
</fieldset>

<style>
  fieldset { min-width: 0; margin: 0; padding: 0; border: 0; }
  legend, label { display: block; margin-bottom: 8px; font-size: 14px; font-weight: 700; }
  .date-inputs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
  input { display: block; width: 100%; min-width: 0; max-width: 100%; min-height: 48px; margin-top: 8px; padding: 8px; border: 1px solid var(--control-line); border-radius: var(--radius); background: transparent; color: var(--ink); }
  .field-error:empty { min-height: 0; margin: 0; }
  @media (max-width: 420px) { .date-inputs { grid-template-columns: minmax(0, 1fr); } }
</style>
